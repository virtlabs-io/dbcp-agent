package agent

import (
	"context"
	"log"
	"time"
)

func Run(ctx context.Context) error {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	log.Println("Agent running...")

	for {
		select {
		case <-ctx.Done():
			log.Println("Agent stopping due to cancellation...")
			return nil
		case <-ticker.C:
			// Future periodic tasks
			log.Println("Agent tick...")
		}
	}
}
