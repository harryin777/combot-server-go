package doubao

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/gorilla/websocket"
)

type AsrWsClient struct {
	seq             int
	segmentDuration int
	url             string
	connect         *websocket.Conn
}

func NewAsrWsClient(url string, segmentDuration int) *AsrWsClient {
	return &AsrWsClient{
		seq:             1,
		url:             url,
		segmentDuration: segmentDuration,
	}
}

func (p *Provider) readAudioData(filePath string) ([]byte, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		log.Fatalf("failed to read file: %s", err)
	}
	isWav := JudgeWav(content)
	if !isWav {
		content, err = ConvertWavWithPath(filePath, DefaultSampleRate)
		if err != nil {
			return nil, fmt.Errorf("convert wav err: %w", err)
		}
	}
	return content, nil
}

func (p *Provider) getSegmentSize(content []byte) (int, error) {
	channelNum, sampWidth, frameRate, _, _, err := ReadWavInfo(content)
	if err != nil {
		return 0, fmt.Errorf("failed to read wav info: %w", err)
	}
	sizePerSec := channelNum * sampWidth * frameRate
	segmentSize := sizePerSec * p.segmentDuration / 1000
	return segmentSize, nil
}

func (p *Provider) createConnection(ctx context.Context) error {
	header := NewAuthHeader(p.accessToken, p.appID)
	conn, resp, err := websocket.DefaultDialer.DialContext(ctx, p.wsURL, header)
	if err != nil {
		return fmt.Errorf("dial websocket err: %w", err)
	}
	log.Printf("logid: %s", resp.Header.Get("X-Tt-Logid"))
	p.conn = conn
	return nil
}

func (p *Provider) sendFullClientRequest() error {
	fullClientRequest := NewFullClientRequest()
	p.seq++
	err := p.conn.WriteMessage(websocket.BinaryMessage, fullClientRequest)
	if err != nil {
		return fmt.Errorf("full client message write websocket err: %w", err)
	}
	_, resp, err := p.conn.ReadMessage()
	if err != nil {
		return fmt.Errorf("full client message read err: %w", err)
	}
	respStruct := ParseResponse(resp)
	log.Println(respStruct)
	return nil
}

func (p *Provider) sendMessages(segmentSize int, content []byte, stopChan <-chan struct{}) error {
	messageChan := make(chan []byte)
	go func() {
		for message := range messageChan {
			err := p.conn.WriteMessage(websocket.TextMessage, message)
			if err != nil {
				log.Printf("write message err: %s", err)
				return
			}
		}
	}()

	audioSegments := splitAudio(content, segmentSize)

	ticker := time.NewTicker(time.Duration(p.segmentDuration) * time.Millisecond)
	defer ticker.Stop()
	defer close(messageChan)
	for _, segment := range audioSegments {
		select {
		case <-ticker.C:
			if p.seq == len(audioSegments)+1 {
				p.seq = -p.seq
			}
			message := NewAudioOnlyRequest(p.seq, segment)
			messageChan <- message
			log.Printf("send message: seq: %d", p.seq)
			p.seq++
		case <-stopChan:
			return nil
		}
	}
	return nil
}

func (p *Provider) recvMessages(resChan chan<- *AsrResponse, stopChan chan<- struct{}) {
	defer close(resChan)
	for {
		_, message, err := p.conn.ReadMessage()
		if err != nil {
			return
		}
		resp := ParseResponse(message)
		resChan <- resp
		if resp.IsLastPackage {
			return
		}
		if resp.Code != 0 {
			close(stopChan)
			return
		}
	}
}

func (p *Provider) startAudioStream(segmentSize int, content []byte, resChan chan<- *AsrResponse) error {
	stopChan := make(chan struct{})
	go func() {
		err := p.sendMessages(segmentSize, content, stopChan)
		if err != nil {
			log.Fatalf("failed to send audio stream: %s", err)
			return
		}
	}()
	p.recvMessages(resChan, stopChan)
	return nil
}

func splitAudio(data []byte, segmentSize int) [][]byte {
	if segmentSize <= 0 {
		return nil // 返回空切片，如果 chunkSize 非法
	}
	var segments [][]byte
	for i := 0; i < len(data); i += segmentSize {
		end := i + segmentSize
		if end > len(data) {
			end = len(data)
		}
		segments = append(segments, data[i:end])
	}
	return segments
}
