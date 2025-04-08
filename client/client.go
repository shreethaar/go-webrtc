package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v3"
)

// Global variables
var (
	peerConnection *webrtc.PeerConnection
	serverConn     *websocket.Conn
	uuid           string
	mutex          sync.Mutex
)

// Signal represents the WebRTC signaling message
type Signal struct {
	SDP  *webrtc.SessionDescription `json:"sdp,omitempty"`
	ICE  *webrtc.ICECandidateInit   `json:"ice,omitempty"`
	UUID string                     `json:"uuid"`
}

func main() {
	// Initialize
	uuid = createUUID()
	log.Printf("Client UUID: %s", uuid)

	// Connect to WebSocket server
	serverURL := "wss://localhost:8443/ws"
	var err error
	serverConn, _, err = websocket.DefaultDialer.Dial(serverURL, nil)
	if err != nil {
		log.Fatalf("Failed to connect to WebSocket server: %v", err)
	}
	defer serverConn.Close()
	log.Println("Connected to signaling server")

	// Configure WebRTC
	config := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{"stun:stun.stunprotocol.org:3478", "stun:stun.l.google.com:19302"},
			},
		},
	}

	// Create media tracks
	// Note: In a real implementation, you would use gstreamer or similar
	// to capture real media, but that's beyond a simple example
	log.Println("Creating media tracks (simulated)")

	// Prepare to handle incoming messages from the server
	go handleServerMessages()

	// Start as a caller (initiator)
	start(true, config)

	// Keep the application running
	select {}
}

func start(isCaller bool, config webrtc.Configuration) {
	var err error
	
	// Create a new PeerConnection
	mutex.Lock()
	peerConnection, err = webrtc.NewPeerConnection(config)
	mutex.Unlock()
	if err != nil {
		log.Fatalf("Failed to create peer connection: %v", err)
	}

	// Set up ICE candidate handling
	peerConnection.OnICECandidate(func(candidate *webrtc.ICECandidate) {
		if candidate == nil {
			return
		}

		candidateInit := candidate.ToJSON()
		signal := Signal{
			ICE:  &candidateInit,
			UUID: uuid,
		}

		sendSignal(signal)
	})

	// Set up track handling
	peerConnection.OnTrack(func(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
		log.Printf("Received remote track: %s", track.ID())
		// In a real implementation, you would handle the media here
	})

	// Create and add a simulated video track (in a real app, this would be a real camera)
	videoTrack, err := webrtc.NewTrackLocalStaticSample(
		webrtc.RTPCodecCapability{MimeType: "video/vp8"},
		"video",
		"pion-video",
	)
	if err != nil {
		log.Fatalf("Failed to create video track: %v", err)
	}
	_, err = peerConnection.AddTrack(videoTrack)
	if err != nil {
		log.Fatalf("Failed to add video track: %v", err)
	}

	// Create and add a simulated audio track
	audioTrack, err := webrtc.NewTrackLocalStaticSample(
		webrtc.RTPCodecCapability{MimeType: "audio/opus"},
		"audio",
		"pion-audio",
	)
	if err != nil {
		log.Fatalf("Failed to create audio track: %v", err)
	}
	_, err = peerConnection.AddTrack(audioTrack)
	if err != nil {
		log.Fatalf("Failed to add audio track: %v", err)
	}

	// Start simulating video frames in a goroutine
	go simulateMediaStream(videoTrack, audioTrack)

	// If this client is the caller, create an offer
	if isCaller {
		createOffer()
	}
}

func createOffer() {
	// Create an offer
	offer, err := peerConnection.CreateOffer(nil)
	if err != nil {
		log.Fatalf("Failed to create offer: %v", err)
	}

	// Set local description
	err = peerConnection.SetLocalDescription(offer)
	if err != nil {
		log.Fatalf("Failed to set local description: %v", err)
	}

	// Send the offer to the signaling server
	signal := Signal{
		SDP:  &offer,
		UUID: uuid,
	}
	sendSignal(signal)
}

func handleServerMessages() {
	for {
		_, message, err := serverConn.ReadMessage()
		if err != nil {
			log.Printf("WebSocket read error: %v", err)
			return
		}

		var signal Signal
		if err := json.Unmarshal(message, &signal); err != nil {
			log.Printf("Failed to parse signal message: %v", err)
			continue
		}

		// Ignore messages from ourselves
		if signal.UUID == uuid {
			continue
		}

		// Handle the signal
		handleSignal(signal)
	}
}

func handleSignal(signal Signal) {
	mutex.Lock()
	pc := peerConnection
	mutex.Unlock()

	if pc == nil {
		// If we don't have a peer connection yet, create one
		config := webrtc.Configuration{
			ICEServers: []webrtc.ICEServer{
				{
					URLs: []string{"stun:stun.stunprotocol.org:3478", "stun:stun.l.google.com:19302"},
				},
			},
		}
		start(false, config)
		mutex.Lock()
		pc = peerConnection
		mutex.Unlock()
	}

	// Handle SDP (offer or answer)
	if signal.SDP != nil {
		err := pc.SetRemoteDescription(*signal.SDP)
		if err != nil {
			log.Printf("Failed to set remote description: %v", err)
			return
		}

		// If we received an offer, create an answer
		if signal.SDP.Type == webrtc.SDPTypeOffer {
			answer, err := pc.CreateAnswer(nil)
			if err != nil {
				log.Printf("Failed to create answer: %v", err)
				return
			}

			err = pc.SetLocalDescription(answer)
			if err != nil {
				log.Printf("Failed to set local description: %v", err)
				return
			}

			answerSignal := Signal{
				SDP:  &answer,
				UUID: uuid,
			}
			sendSignal(answerSignal)
		}
	}

	// Handle ICE candidate
	if signal.ICE != nil {
		err := pc.AddICECandidate(*signal.ICE)
		if err != nil {
			log.Printf("Failed to add ICE candidate: %v", err)
			return
		}
	}
}

func sendSignal(signal Signal) {
	data, err := json.Marshal(signal)
	if err != nil {
		log.Printf("Failed to marshal signal: %v", err)
		return
	}

	err = serverConn.WriteMessage(websocket.TextMessage, data)
	if err != nil {
		log.Printf("Failed to send signal: %v", err)
		return
	}
}

// simulateMediaStream simulates sending video and audio frames
func simulateMediaStream(videoTrack, audioTrack *webrtc.TrackLocalStaticSample) {
	// In a real application, this would capture from a camera and microphone
	// For this example, we'll just simulate sending frames
	ticker := time.NewTicker(33 * time.Millisecond) // ~30fps
	for range ticker.C {
		// Create dummy video frame
		videoSample := &webrtc.Sample{
			Data:     make([]byte, 640*480*3), // RGB data
			Duration: 33 * time.Millisecond,
		}
		// Fill with random data to simulate changing video
		rand.Read(videoSample.Data)
		
		if err := videoTrack.WriteSample(*videoSample); err != nil {
			log.Printf("Failed to write video sample: %v", err)
		}

		// Create dummy audio sample
		audioSample := &webrtc.Sample{
			Data:     make([]byte, 1024), // Audio data
			Duration: 33 * time.Millisecond,
		}
		rand.Read(audioSample.Data)
		
		if err := audioTrack.WriteSample(*audioSample); err != nil {
			log.Printf("Failed to write audio sample: %v", err)
		}
	}
}

// createUUID generates a UUID v4-like string
func createUUID() string {
	rand.Seed(time.Now().UnixNano())
	return fmt.Sprintf("%04x%04x-%04x-%04x-%04x-%04x%04x%04x",
		rand.Intn(0x10000), rand.Intn(0x10000),
		rand.Intn(0x10000),
		rand.Intn(0x10000),
		rand.Intn(0x10000),
		rand.Intn(0x10000), rand.Intn(0x10000), rand.Intn(0x10000))
}
