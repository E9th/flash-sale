package internal

import (
	"fmt"
	"log"
	"os"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var DB *gorm.DB

func ConnectDB() {
	host := os.Getenv("DB_HOST")
	if host == "" {
		host = "127.0.0.1"
	}

	dsn := fmt.Sprintf("host=%s user=admin password=password123 dbname=flashsale_db port=5432 sslmode=disable", host)

	// --- 🔄 Retry Logic Start ---
	counts := 0
	for {
		var err error
		DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
		if err != nil {
			log.Printf("Postgres not ready... waiting (Attempt %d)", counts)
			counts++
		} else {
			log.Println("✅ Connected to Database!")
			return // ต่อติดแล้ว ออกจากฟังก์ชันได้
		}

		if counts > 10 { // ลองครบ 10 ครั้ง (20 วิ) แล้วยังไม่ได้ ก็ยอมแพ้
			log.Panic("Failed to connect to database after retries:", err)
		}

		log.Println("Backing off for 2 seconds...")
		time.Sleep(2 * time.Second) // รอ 2 วินาทีก่อนลองใหม่
		continue
	}
	// --- 🔄 Retry Logic End ---
}

// Structs แทนตารางใน DB
type Product struct {
	ID       uint
	Name     string
	Quantity int
}

type Order struct {
	ID        uint
	UserID    int
	ProductID int
}
