package main

import (
	"encoding/json"
	"flash-sale-system/internal"
	"fmt"
	"log"

	"gorm.io/gorm"
)

func main() {
	// 1. เชื่อมต่อระบบ
	internal.ConnectDB()
	internal.ConnectRedis() // <--- ต้องต่อ Redis ด้วย
	internal.ConnectRabbitMQ()
	defer internal.RabbitConn.Close()
	defer internal.RabbitCh.Close()

	fmt.Println("👷 Worker started. Waiting for messages...")

	// 2. รับข้อความจาก Queue
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

			// --- 3. เริ่ม Transaction (Database) ---
			tx := internal.DB.Begin()

			// 3.1 สร้าง Order
			order := internal.Order{
				UserID:    orderData.UserID,
				ProductID: orderData.ProductID,
			}
			if err := tx.Create(&order).Error; err != nil {
				tx.Rollback()
				fmt.Printf("❌ Failed to create order: %v\n", err)
				continue
			}

			// 3.2 ตัด Stock ใน DB
			if err := tx.Model(&internal.Product{}).
				Where("id = ?", orderData.ProductID).
				Update("quantity", gorm.Expr("quantity - ?", 1)).Error; err != nil {

				tx.Rollback()
				fmt.Printf("❌ Failed to update stock: %v\n", err)
				continue
			}

			tx.Commit()

			// --- 4. สำคัญมาก! แจ้งข่าวผ่าน Redis (Real-time) ---
			var currentStock int
			// ดึงค่า Stock ล่าสุดจาก DB เพื่อความชัวร์
			internal.DB.Model(&internal.Product{}).Select("quantity").Where("id = ?", orderData.ProductID).Scan(&currentStock)

			// ส่งข้อความ JSON บอก API ว่า "เหลือเท่าไหร่แล้ว"
			msg := fmt.Sprintf(`{"product_id": %d, "stock": %d}`, orderData.ProductID, currentStock)
			internal.RDB.Publish(internal.Ctx, "stock_updates", msg)

			fmt.Printf("✅ Processed Order %d | Stock: %d | 📢 Broadcast sent\n", order.ID, currentStock)
		}
	}()

	<-forever
}
