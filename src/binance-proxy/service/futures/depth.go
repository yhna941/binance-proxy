package futures

import (
	"log"
	"sync"
	"time"

	client "github.com/adshao/go-binance/v2/futures"
)

type FuturesDepth struct {
	mutex sync.RWMutex

	stopC chan struct{}
	si    SymbolInterval
	depth *client.DepthResponse
}

func NewFutresDepth(si SymbolInterval) *FuturesDepth {
	return &FuturesDepth{si: si, stopC: make(chan struct{})}
}

func (s *FuturesDepth) Start() {
	go func() {
		loop := 1
		for {
			s.mutex.Lock()
			s.depth = nil
			s.mutex.Unlock()

			client.WebsocketKeepalive = true
			doneC, stopC, err := client.WsPartialDepthServeWithRate(s.si.Symbol, 20, 100*time.Millisecond, s.wsHandler, s.errHandler)
			if err == nil {
				loop = 1
				select {
				case stopC <- <-s.stopC:
					return
				case <-doneC:
				}
			}

			time.Sleep(time.Second * time.Duration(loop))

			if loop < 60 {
				loop *= 2
			}
		}
	}()
}

func (s *FuturesDepth) Stop() {
	s.stopC <- struct{}{}
}

func (s *FuturesDepth) wsHandler(event *client.WsDepthEvent) {
	defer s.mutex.Unlock()
	s.mutex.Lock()

	s.depth = &client.DepthResponse{
		LastUpdateID: event.LastUpdateID,
		Bids:         event.Bids,
		Asks:         event.Asks,
	}
}

func (s *FuturesDepth) errHandler(err error) {
	log.Printf("%s.Depth websocket throw error!Error:%s", s.si, err)
}

func (s *FuturesDepth) GetDepth() client.DepthResponse {
	defer s.mutex.RUnlock()
	s.mutex.RLock()

	var t client.DepthResponse
	if s.depth != nil {
		t = *s.depth
	}

	return t
}
