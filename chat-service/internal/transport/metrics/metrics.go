package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	MessagesTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "chat_messages_total",
		Help: "Total messages sent",
	}, []string{"conversation_id"})

	ActiveConnections = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "chat_websocket_active_connections",
		Help: "Number of active WebSocket connections",
	})

	ConversationsCreated = promauto.NewCounter(prometheus.CounterOpts{
		Name: "chat_conversations_created_total",
		Help: "Total conversations created",
	})
)
