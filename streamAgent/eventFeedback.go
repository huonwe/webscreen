package sagent

import (
	"log"
	"webcpy/sdriver"
)

func (sa *Agent) EventFeedback(yield func([]byte) bool) {
	for event := range sa.controlCh {
		// log.Printf("[Agent] Received event: %+v", event)
		eType := event.Type()
		switch eType {
		case sdriver.EVENT_TYPE_RECEIVE_CLIPBOARD:
			event := event.(sdriver.SampleEvent)
			content := event.GetContent()
			msg := make([]byte, 1+len(content))
			copy(msg[1:], content)
			msg[0] = byte(sdriver.EVENT_TYPE_RECEIVE_CLIPBOARD)
			if !yield(msg) {
				return
			}
		default:
			log.Printf("Unhandled event type in ReceiveEvent: %d", eType)
		}
	}
}
