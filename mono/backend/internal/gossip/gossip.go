package gossip

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

// Message is a signed gossip message exchanged between nodes.
type Message struct {
	SenderID  string    `json:"sender_id"`
	Payload   string    `json:"payload"`
	Timestamp time.Time `json:"timestamp"`
	Nonce     string    `json:"nonce"`
	Signature string    `json:"signature"`
}

// Node represents a gossip peer with its public key.
type Node struct {
	ID         string
	PublicKey  ed25519.PublicKey
	LastSeen   time.Time
	Reputation float64
}

// Protocol handles Ed25519-signed gossip messages.
type Protocol struct {
	mu         sync.RWMutex
	nodeID     string
	privateKey ed25519.PrivateKey
	publicKey  ed25519.PublicKey
	nodes      map[string]*Node
}

// NewProtocol creates a new gossip protocol instance with a generated Ed25519 key pair.
func NewProtocol(nodeID string) (*Protocol, error) {
	pubKey, privKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("gossip key generation failed: %w", err)
	}
	return &Protocol{
		nodeID:     nodeID,
		privateKey: privKey,
		publicKey:  pubKey,
		nodes:      make(map[string]*Node),
	}, nil
}

// PublicKeyHex returns the node's public key as hex.
func (p *Protocol) PublicKeyHex() string {
	return hex.EncodeToString(p.publicKey)
}

// Sign creates a signed gossip message.
func (p *Protocol) Sign(payload string) (*Message, error) {
	nonce := make([]byte, 16)
	rand.Read(nonce)

	msg := &Message{
		SenderID:  p.nodeID,
		Payload:   payload,
		Timestamp: time.Now(),
		Nonce:     hex.EncodeToString(nonce),
	}

	// Sign the message content
	data := []byte(fmt.Sprintf("%s:%s:%d:%s", msg.SenderID, msg.Payload, msg.Timestamp.UnixNano(), msg.Nonce))
	sig := ed25519.Sign(p.privateKey, data)
	msg.Signature = hex.EncodeToString(sig)

	return msg, nil
}

// Verify checks a message's signature and updates reputation.
func (p *Protocol) Verify(msg *Message, senderPubKeyHex string) bool {
	pubKeyBytes, err := hex.DecodeString(senderPubKeyHex)
	if err != nil || len(pubKeyBytes) != ed25519.PublicKeySize {
		return false
	}

	sig, err := hex.DecodeString(msg.Signature)
	if err != nil {
		return false
	}

	data := []byte(fmt.Sprintf("%s:%s:%d:%s", msg.SenderID, msg.Payload, msg.Timestamp.UnixNano(), msg.Nonce))
	valid := ed25519.Verify(ed25519.PublicKey(pubKeyBytes), data, sig)

	// Reject messages older than 30 seconds (anti-replay)
	if time.Since(msg.Timestamp) > 30*time.Second {
		return false
	}

	return valid
}

// AddNode registers a peer node.
func (p *Protocol) AddNode(id string, pubKeyHex string) error {
	pubKeyBytes, err := hex.DecodeString(pubKeyHex)
	if err != nil {
		return err
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	p.nodes[id] = &Node{
		ID:         id,
		PublicKey:  pubKeyBytes,
		LastSeen:   time.Now(),
		Reputation: 1.0,
	}
	return nil
}

// GetNode returns a registered node by ID.
func (p *Protocol) GetNode(id string) *Node {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.nodes[id]
}

// UpdateReputation adjusts a node's reputation score.
func (p *Protocol) UpdateReputation(nodeID string, delta float64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if node, ok := p.nodes[nodeID]; ok {
		node.Reputation += delta
		if node.Reputation < 0 {
			node.Reputation = 0
		}
		if node.Reputation > 1.0 {
			node.Reputation = 1.0
		}
	}
}
