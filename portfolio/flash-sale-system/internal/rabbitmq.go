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

	var err error
	// เชื่อมต่อ RabbitMQ (Default port 5672)
	dsn := fmt.Sprintf("amqp://admin:password123@%s:5672/", host)
	RabbitConn, err = amqp.Dial(dsn)
	failOnError(err, "Failed to connect to RabbitMQ")

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
