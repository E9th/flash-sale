package main

import (
	"context"
	"flash-sale-system/internal"
	"fmt"
	"log"
	"sync"

	"github.com/gofiber/contrib/websocket" // <--- Library ใหม่
	"github.com/gofiber/fiber/v2"
)

// ตัวแปรเก็บรายชื่อคนที่ต่อ WebSocket เข้ามา (Clients)
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

	// (Phase 2 Code: seedStock() เอาไว้เหมือนเดิม)
	seedStock()

	app := fiber.New()

	// --- 1. WebSocket Endpoint ---
	// ต้องมี middleware เช็คก่อนว่าเป็นการเชื่อมต่อแบบ WS หรือไม่
	app.Use("/ws", func(c *fiber.Ctx) error {
		if websocket.IsWebSocketUpgrade(c) {
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	})

	app.Get("/ws", websocket.New(func(c *websocket.Conn) {
		// เมื่อมีคนต่อเข้ามา ให้เก็บ connection ไว้ใน map
		clientsMu.Lock()
		clients[c] = true
		clientsMu.Unlock()

		log.Println("🟢 New WebSocket Client Connected")

		// รอจนกว่าเขาจะตัดสาย
		defer func() {
			clientsMu.Lock()
			delete(clients, c)
			clientsMu.Unlock()
			c.Close()
			log.Println("🔴 Client Disconnected")
		}()

		// Loop ฟังข้อความจาก Client (ถึงเราจะไม่ได้รับอะไร แต่ต้อง Loop ไว้ไม่งั้น Connection หลุด)
		for {
			if _, _, err := c.ReadMessage(); err != nil {
				break
			}
		}
	}))

	// --- 2. Background Task: ฟัง Redis แล้วกระจายข่าว (Broadcaster) ---
	go func() {
		ctx := context.Background()
		// Subscribe ช่อง "stock_updates"
		pubsub := internal.RDB.Subscribe(ctx, "stock_updates")
		defer pubsub.Close()

		ch := pubsub.Channel()

		// วนลูปรับข้อความจาก Redis
		for msg := range ch {
			// พอได้ข่าวมา ก็ส่งต่อให้ Clients ทุกคน
			clientsMu.Lock()
			for client := range clients {
				if err := client.WriteMessage(websocket.TextMessage, []byte(msg.Payload)); err != nil {
					client.Close()
					delete(clients, client)
				}
			}
			clientsMu.Unlock()
		}
	}()

	// --- 3. Serve หน้า Dashboard (Frontend) ---
	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendFile("index.html") // เดี๋ยวเราสร้างไฟล์นี้กัน
	})

	// API Buy เดิม (เอาไว้เหมือนเดิม)
	app.Post("/api/buy", func(c *fiber.Ctx) error {
		// ... (Code เดิมทั้งหมดของ Phase 3/4) ...
		// (Copy Code เดิมจาก Phase 3 มาใส่ตรงนี้ได้เลยครับ หรือถ้าไฟล์เดิมมีอยู่แล้วก็ไม่ต้องแก้ส่วนนี้)
		return c.SendStatus(200) // Placeholder
	})

	// หมายเหตุ: อย่าลืม copy logic ของ /api/buy กลับมาใส่นะครับ เดี๋ยวซื้อไม่ได้ 😅
	// หรือถ้าไม่อยากแก้เยอะ ให้แปะ Code WebSocket แทรกเข้าไประหว่าง app := fiber.New() กับ app.Post() ครับ

	app.Listen(":8080")
}

func seedStock() {
	// (ใช้ code เดิมจาก Phase 2)
	var product internal.Product
	if err := internal.DB.First(&product, 1).Error; err != nil {
		return
	}
	key := fmt.Sprintf("product:%d:stock", product.ID)
	internal.RDB.Set(internal.Ctx, key, product.Quantity, 0)
	fmt.Printf("✅ Seeded Redis: %s = %d\n", key, product.Quantity)
}
