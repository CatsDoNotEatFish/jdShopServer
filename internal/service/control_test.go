package service

import (
	"testing"
	"time"

	"jdShopServer/internal/model"
)

func TestControlHubBroadcastsAnnouncementToAllConnectedUsers(t *testing.T) {
	hub := NewControlHub()
	first, stopFirst := hub.Subscribe(10)
	defer stopFirst()
	second, stopSecond := hub.Subscribe(20)
	defer stopSecond()

	hub.Broadcast(model.ControlEvent{
		Type:           "announcement_published",
		AnnouncementID: 7,
		Revision:       3,
	})

	for name, channel := range map[string]<-chan model.ControlEvent{"first": first, "second": second} {
		select {
		case event := <-channel:
			if event.Type != "announcement_published" || event.AnnouncementID != 7 || event.Revision != 3 || event.IssuedAt == "" {
				t.Fatalf("%s subscriber received unexpected event: %+v", name, event)
			}
		case <-time.After(time.Second):
			t.Fatalf("%s subscriber did not receive broadcast", name)
		}
	}
}
