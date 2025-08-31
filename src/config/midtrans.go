package config

import (
	"log"
	"os"
	midtrans "github.com/midtrans/midtrans-go"
	"github.com/midtrans/midtrans-go/coreapi"
	"github.com/joho/godotenv"
)

var MidtransClient = coreapi.Client{}

func InitMidtrans() {
    err := godotenv.Load()
    if err != nil {
        log.Println("No .env file found")
    }

    serverKey := os.Getenv("MIDTRANS_SERVER_KEY")
    env := os.Getenv("MIDTRANS_ENV")

    MidtransClient.New(serverKey, midtrans.Sandbox)
    if env == "production" {
        MidtransClient.New(serverKey, midtrans.Production)
    }
}