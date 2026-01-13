package main

import (
	"context"
	"encoding/json"
	"flash-sale-system/internal"
	"fmt"
	"sync"
	"time"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
)

// ตัวแปรเก็บรายชื่อคนที่ต่อ WebSocket เข้ามา
var (
	clients   = make(map[*websocket.Conn]bool)
	clientsMu sync.Mutex
)

func main() {
	internal.ConnectDB()
	internal.ConnectRedis()
	internal.ConnectRabbitMQ()
	defer internal.RabbitConn.Close()
	defer internal.RabbitCh.Close()

	seedStock()

	app := fiber.New()

	// --- 1. WebSocket Endpoint ---
	app.Use("/ws", func(c *fiber.Ctx) error {
		if websocket.IsWebSocketUpgrade(c) {
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	})

	app.Get("/ws", websocket.New(func(c *websocket.Conn) {
		clientsMu.Lock()
		clients[c] = true
		clientsMu.Unlock()

		defer func() {
			clientsMu.Lock()
			delete(clients, c)
			clientsMu.Unlock()
			c.Close()
		}()

		for {
			if _, _, err := c.ReadMessage(); err != nil {
				break
			}
		}
	}))

	// --- 2. Background Task: ฟัง Redis แล้วกระจายข่าว ---
	go func() {
		ctx := context.Background()
		// Retry Logic สำหรับ Subscribe (เผื่อ Redis ยังไม่ตื่น)
		for {
			pubsub := internal.RDB.Subscribe(ctx, "stock_updates")
			ch := pubsub.Channel()

			// Test connection
			_, err := pubsub.Receive(ctx)
			if err != nil {
				fmt.Println("Redis PubSub not ready, retrying...")
				time.Sleep(2 * time.Second)
				continue
			}

			fmt.Println("🎧 API is listening to stock updates...")

			for msg := range ch {
				clientsMu.Lock()
				for client := range clients {
					if err := client.WriteMessage(websocket.TextMessage, []byte(msg.Payload)); err != nil {
						client.Close()
						delete(clients, client)
					}
				}
				clientsMu.Unlock()
			}
		}
	}()

	// --- 3. Serve Frontend ---
	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendFile("index.html")
	})

	// --- 4. Logic ซื้อของ (เอาของเดิมกลับมาใส่!) ---
	app.Post("/api/buy", func(c *fiber.Ctx) error {
		type Request struct {
			UserID    int `json:"user_id"`
			ProductID int `json:"product_id"`
		}
		var req Request
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid request"})
		}

		// 4.1 Redis Check
		stockKey := fmt.Sprintf("product:%d:stock", req.ProductID)
		remaining, err := internal.RDB.Decr(internal.Ctx, stockKey).Result()
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Redis error"})
		}
		if remaining < 0 {
			return c.Status(400).JSON(fiber.Map{"error": "Sold out!"})
		}

		// 4.2 RabbitMQ Publish
		orderData, _ := json.Marshal(map[string]interface{}{
			"user_id":    req.UserID,
			"product_id": req.ProductID,
			"timestamp":  time.Now().Unix(),
		})

		err = internal.PublishToQueue(orderData)
		if err != nil {
			internal.RDB.Incr(internal.Ctx, stockKey) // คืนของ
			return c.Status(500).JSON(fiber.Map{"error": "Failed to queue order"})
		}

		// 4.3 Response (202 Accepted)
		return c.Status(202).JSON(fiber.Map{
			"message":         "Order received, processing in background",
			"remaining_stock": remaining,
			"status":          "queued",
		})
	})

	app.Listen(":8080")
}

func seedStock() {
	var product internal.Product
	if err := internal.DB.First(&product, 1).Error; err != nil {
		return
	}
	key := fmt.Sprintf("product:%d:stock", product.ID)
	internal.RDB.Set(internal.Ctx, key, product.Quantity, 0)
	fmt.Printf("✅ Seeded Redis: %s = %d\n", key, product.Quantity)
}
