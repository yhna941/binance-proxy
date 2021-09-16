package futures

import (
	"container/list"
	"context"
	"log"
	"sync"
	"time"

	client "github.com/adshao/go-binance/v2/futures"
)

type FuturesKlines struct {
	mutex sync.RWMutex

	stopC  chan struct{}
	si     SymbolInterval
	klines *list.List
}

func NewFutresKlines(si SymbolInterval) *FuturesKlines {
	return &FuturesKlines{si: si, stopC: make(chan struct{})}
}

func (s *FuturesKlines) Start() {
	go func() {
		loop := 1
		for {
			s.mutex.Lock()
			s.klines = nil
			s.mutex.Unlock()

			client.WebsocketKeepalive = true
			doneC, stopC, err := client.WsKlineServe(s.si.Symbol, s.si.Interval, s.wsHandler, s.errHandler)
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

func (s *FuturesKlines) Stop() {
	s.stopC <- struct{}{}
}

func (s *FuturesKlines) wsHandler(event *client.WsKlineEvent) {
	defer s.mutex.Unlock()
	s.mutex.Lock()

	if s.klines == nil {
		loop := 1
		for {
			klines, err := client.NewClient("", "").NewKlinesService().
				Symbol(s.si.Symbol).Interval(s.si.Interval).Limit(1500).
				Do(context.Background())
			if err == nil {
				s.klines = list.New()
				for v := range klines {
					s.klines.PushBack(v)
				}
				break
			}

			log.Printf("%s.Get initialization klines error!Error:%s", s.si, err)
			time.Sleep(time.Second * time.Duration(loop))

			if loop < 60 {
				loop *= 2
			}
		}
	}

	// Merge kline
	kline := &client.Kline{
		OpenTime:                 event.Kline.StartTime,
		Open:                     event.Kline.Open,
		High:                     event.Kline.High,
		Low:                      event.Kline.Low,
		Close:                    event.Kline.Close,
		Volume:                   event.Kline.Volume,
		CloseTime:                event.Kline.EndTime,
		QuoteAssetVolume:         event.Kline.QuoteVolume,
		TradeNum:                 event.Kline.TradeNum,
		TakerBuyBaseAssetVolume:  event.Kline.ActiveBuyVolume,
		TakerBuyQuoteAssetVolume: event.Kline.ActiveBuyQuoteVolume,
	}

	if s.klines.Back().Value.(*client.Kline).OpenTime < kline.OpenTime {
		s.klines.PushBack(kline)
	} else if s.klines.Back().Value.(*client.Kline).OpenTime == kline.OpenTime {
		s.klines.Back().Value = kline
	}

	for s.klines.Len() > 1000 {
		s.klines.Remove(s.klines.Front())
	}
}

func (s *FuturesKlines) errHandler(err error) {
	log.Printf("%s.Kline websocket throw error!Error:%s", s.si, err)
}

func (s *FuturesKlines) GetKlines() []client.Kline {
	defer s.mutex.RUnlock()
	s.mutex.RLock()

	res := make([]client.Kline, s.klines.Len())

	for elems := s.klines.Front(); elems != nil; elems = elems.Next() {
		res = append(res, *(elems.Value.(*client.Kline)))
	}

	return res
}
