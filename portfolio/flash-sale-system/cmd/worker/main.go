package main

import (
	"encoding/json"
	"flash-sale-system/internal"
	"fmt"
	"log"

	"gorm.io/gorm" // <--- อย่าลืม import gorm
)

func main() {
	internal.ConnectDB()
	internal.ConnectRabbitMQ()
	// Add Redis connection if not already handled inside 'internal' package init or similar,
	// assuming internal.RDB is available and connected based on the prompt's usage.
	// If connect is explicit: internal.ConnectRedis() (Checking assumed context, usually implied)

	defer internal.RabbitConn.Close()
	defer internal.RabbitCh.Close()

	fmt.Println("👷 Worker started. Waiting for messages...")

	msgs, err := internal.RabbitCh.Consume(
		internal.QueueName, "", true, false, false, false, nil,
	)
	if err != nil {
		log.Fatal(err)
	}

	forever := make(chan struct{})

	go func() {
		for d := range msgs {
			var orderData struct {
				UserID    int `json:"user_id"`
				ProductID int `json:"product_id"`
			}
			json.Unmarshal(d.Body, &orderData)

			// --- เริ่ม Transaction ---
			tx := internal.DB.Begin()

			// 1. สร้าง Order
			order := internal.Order{
				UserID:    orderData.UserID,
				ProductID: orderData.ProductID,
			}
			if err := tx.Create(&order).Error; err != nil {
				tx.Rollback()
				fmt.Printf("❌ Failed to create order: %v\n", err)
				continue
			}

			// 2. ตัด Stock ใน DB (SQL: UPDATE products SET quantity = quantity - 1 WHERE id = ?)
			// ใช้ gorm.Expr เพื่อลดค่าลง 1
			if err := tx.Model(&internal.Product{}).
				Where("id = ?", orderData.ProductID).
				Update("quantity", gorm.Expr("quantity - ?", 1)).Error; err != nil {

				tx.Rollback()
				fmt.Printf("❌ Failed to update stock: %v\n", err)
				continue
			}

			// 3. Commit (ยืนยันทั้งคู่)
			tx.Commit()

			// Log แบบสั้นๆ จะได้ดูง่ายๆ
			fmt.Printf("✅ Processed: OrderID %d | Stock updated\n", order.ID)

			// --- ส่วนที่เพิ่มใหม่: แจ้งข่าวผ่าน Redis Pub/Sub ---
			// หาจำนวนของล่าสุด (Optional: หรือจะส่งแค่ว่าลบไป 1 ก็ได้ แต่ส่งยอดคงเหลือชัวร์กว่า)
			var currentStock int
			internal.DB.Model(&internal.Product{}).Select("quantity").Where("id = ?", orderData.ProductID).Scan(&currentStock)

			// Publish ข้อความไปที่ Channel "stock_updates"
			// Payload: JSON string ง่ายๆ เช่น {"product_id":1, "stock": 99}
			msg := fmt.Sprintf(`{"product_id": %d, "stock": %d}`, orderData.ProductID, currentStock)
			internal.RDB.Publish(internal.Ctx, "stock_updates", msg)

			// Log
			fmt.Printf("📢 Published Update: %s\n", msg)
		}
	}()

	<-forever
}
