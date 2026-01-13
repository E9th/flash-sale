package internal

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

var (
	RabbitConn *amqp.Connection
	RabbitCh   *amqp.Channel
	QueueName  = "buy_orders"
)

func ConnectRabbitMQ() {
	host := os.Getenv("RABBITMQ_HOST")
	if host == "" {
		host = "localhost"
	}

	dsn := fmt.Sprintf("amqp://admin:password123@%s:5672/", host)
	var err error

	// --- 🔄 Retry Logic Start ---
	counts := 0
	for {
		RabbitConn, err = amqp.Dial(dsn)
		if err != nil {
			fmt.Printf("RabbitMQ not ready... waiting (Attempt %d)\n", counts)
			counts++
		} else {
			fmt.Println("✅ Connected to RabbitMQ!")
			break // ต่อติดแล้ว ออกจาก Loop
		}

		if counts > 15 { // รอสูงสุด 30 วิ (15 * 2s)
			log.Panicf("Failed to connect to RabbitMQ after retries: %s", err)
		}

		time.Sleep(2 * time.Second)
		continue
	}
	// --- 🔄 Retry Logic End ---

	RabbitCh, err = RabbitConn.Channel()
	failOnError(err, "Failed to open a channel")

	// ประกาศ Queue (ถ้ายังไม่มี มันจะสร้างให้)
	_, err = RabbitCh.QueueDeclare(
		QueueName, // name
		true,      // durable (save ลง disk กันไฟดับ)
		false,     // delete when unused
		false,     // exclusive
		false,     // no-wait
		nil,       // arguments
	)
	failOnError(err, "Failed to declare a queue")

	fmt.Println("Connection Opened to RabbitMQ")
}

// ฟังก์ชันส่งข้อความ (Publish)
func PublishToQueue(body []byte) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return RabbitCh.PublishWithContext(ctx,
		"",        // exchange
		QueueName, // routing key
		false,     // mandatory
		false,     // immediate
		amqp.Publishing{
			ContentType: "application/json",
			Body:        body,
			// DeliveryMode: 2 คือ Persistent (บันทึกลง Disk)
			DeliveryMode: amqp.Persistent,
		},
	)
}

func failOnError(err error, msg string) {
	if err != nil {
		log.Panicf("%s: %s", msg, err)
	}
}
