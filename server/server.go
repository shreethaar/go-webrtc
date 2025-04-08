package main
import (
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/pion/webrtc/v4"
	"golang.org/x/net/websocket"
)


const httpsPort = 8443

var (
	upgrader=websocket.Upgrader {
		CheckOrigin:func(r *http.Request) bool {
			return true
		},
	}
	clients=make(map[*websocket.Conn]bool)
)

func websocketHandler(c echo.Context) error {
	ws, err := upgrader.Upgrade(c.Response(),c.Request(),nil)
	if err!=nil {
		log.Println("websocket upgrade error:",err)
		return err
	}
	defer ws.Close()
	
	clients[ws]=true
	log.Println("Client connected via websocket")

	for {
		_,message,err:=ws.ReadMessage() 
		if err!=nil {
			log.Println("read error:",err)
			delete(clients,ws)
			break 
		}
		log.Printf("Received: %s", message)
		broadcastMessage(message)
		/*if err := ws.WriteMessage(messageType, message); err != nil {
			log.Println("write error:", err)
			break
		*/
	}
	return nil
}
		
// broadcast message to all connected clients
func broadcastMessage(message []byte) {
	for client:=range clients {
		err:=client.WriteMessage(websocket.TextMessage,message) 
		if err!=nil {
			log.Println("write error:",err)
			client.Close()
			delete(clients,client)
		}
	}
}


func main() {
	e:=echo.New() 
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.GET("/",func(c echo.Context) error {
		return c.File("client/index.html")
	})
	e.GET("/webrtc.js", func(c echo.Context) error {
		return c.File("client/webrtc.js")
	})
	e.GET("/ws",websocketHandler)
	
	/*
	tls.Config:=&tls.Config{
		MinVersion: tls.VersionTLS13,
	}

	server:=&http.Server {
		Addr:	httpsPort,
		TLSConfig: tlsConfig,
	}
	*/
	if _, err := os.Stat("cert.pem"); os.IsNotExist(err) {
		log.Fatal("cert.pem file not found")
	}
	if _, err := os.Stat("key.pem"); os.IsNotExist(err) {
		log.Fatal("key.pem file not found")
	}
	/*
	log.Printf("Starting Echo HTTPS server on port %s...", httpsPort)
	if err := e.StartTLS(httpsPort, "cert.pem", "key.pem"); err != nil {
		log.Fatal("Server failed to start:", err)
	}
	*/
	printHelp()
	
	// Start HTTPS server
	if err := e.StartTLS(":"+httpsPort, "cert.pem", "key.pem"); err != nil {
		log.Fatal("Server failed to start:", err)
	}
}

func printHelp() {
	fmt.Printf("Server running. Visit https://localhost:%s in Firefox/Chrome/Safari.\n\n", httpsPort)
	fmt.Println("Please note the following:")
	fmt.Println("  * Note the HTTPS in the URL; there is no HTTP -> HTTPS redirect.")
	fmt.Println("  * You'll need to accept the invalid TLS certificate as it is self-signed.")
	fmt.Println("  * Some browsers or OSs may not allow the webcam to be used by multiple pages at once. You may need to use two different browsers or machines.")
}
