package main

import (
    "log"
    "patreonoj/internal/config"
    "patreonoj/internal/models"
)

// Share initialization with the server to keep migrations and logging consistent.
func main() {
    if err := config.InitConfig("config.yaml"); err != nil { log.Fatal(err) }
    if err := models.InitDB(); err != nil { log.Fatal(err) }
    if db, err := models.DB.DB(); err == nil { defer db.Close() }
    if err := models.AutoMigrate(); err != nil { log.Fatal(err) }
}