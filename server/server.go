package server

import (
	"crypto/tls"
	"fmt"
	"log"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/pion/webrtc/v4"
)


const httpsPort = 8443


func websocketHandler(c echo.Context) error {
	ws, err := upgrader.Upgrade(c.Response(),c.Request(),nil)
	if err!=nil {
		log.Println("websocket upgrade error:",err)
		return err
	}
	defer ws.Close()
	log.Println("Client connected via websocket")

	for {
		messageType,message,err:=ws.ReadMessage() 
		if err!=nil {
			log.Println("read error:",err)
			break 
		}
		log.Printf("Received: %s", message)

		if err := ws.WriteMessage(messageType, message); err != nil {
			log.Println("write error:", err)
			break
		}
	}
	return nil
}
		

func main() {
	e:=echo.New() 
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.GET("/ws",websocketHandler)

	tls.Config:=&tls.Config{
		MinVersion: tls.VersionTLS13,
	}

	server:=&http.Server {
		Addr:	httpsPort,
		TLSConfig: tlsConfig,
	}

	log.Printf("Starting Echo HTTPS server on port %s...", httpsPort)
	if err := e.StartTLS(httpsPort, "cert.pem", "key.pem"); err != nil {
		log.Fatal("Server failed to start:", err)
	}
}
