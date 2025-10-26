package doubao

import (
	"combot-server-go/src/log"
	"context"
	"fmt"
	"os"
	"time"

	"github.com/gorilla/websocket"
)

type AsrWsClient struct {
	seq             int
	segmentDuration int
	url             string
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
		//log.Fatalf("failed to read file: %s", err)
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
	p.connMutex.Lock()
	defer p.connMutex.Unlock()

	// 如果连接已存在且处于正常状态，直接返回
	if p.conn != nil && p.isStreaming {
		return nil
	}

	// 清理旧连接
	if p.conn != nil {
		p.closeConnection()
	}

	// 建立WebSocket连接
	header := NewAuthHeader(p.accessToken, p.appID)

	// 创建一个不使用代理的 Dialer
	dialer := &websocket.Dialer{
		Proxy:            nil, // 禁用代理
		HandshakeTimeout: 15 * time.Second,
	}

	log.Infof(ctx, "正在连接豆包ASR服务: %s", p.wsURL)

	// 使用新的 context，避免使用已取消的 context
	dialCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	conn, resp, err := dialer.DialContext(dialCtx, p.wsURL, header)
	if err != nil {
		log.Errorf(ctx, "WebSocket连接失败: %v", err)
		return fmt.Errorf("dial websocket err: %w", err)
	}

	log.Infof(ctx, "WebSocket连接已建立，响应状态码: %d, logid: %s",
		resp.StatusCode, resp.Header.Get("X-Tt-Logid"))

	p.conn = conn

	// 重置序列号，从 1 开始
	p.seq = 1

	// 发送 FullClientRequest (seq=1)
	fullClientRequest := NewFullClientRequest()
	if err := p.conn.WriteMessage(websocket.BinaryMessage, fullClientRequest); err != nil {
		p.closeConnection()
		return fmt.Errorf("发送初始化请求失败: %w", err)
	}

	log.Infof(ctx, "已发送初始化请求(seq=%d)，等待响应...", p.seq)

	// 序列号递增，下一个音频包从 2 开始
	p.seq++

	// 读取初始化响应
	_, respData, err := p.conn.ReadMessage()
	if err != nil {
		p.closeConnection()
		return fmt.Errorf("读取初始化响应失败: %w", err)
	}

	respStruct := ParseResponse(respData)

	log.Infof(ctx, "收到初始化响应: Code=%d, IsLastPackage=%v",
		respStruct.Code, respStruct.IsLastPackage)

	if respStruct.Code != 0 {
		errMsg := "未知错误"
		if respStruct.PayloadMsg != nil {
			errMsg = respStruct.PayloadMsg.Error
		}
		p.closeConnection()
		return fmt.Errorf("ASR初始化错误: Code=%d, Message=%s", respStruct.Code, errMsg)
	}

	log.Infof(ctx, "ASR初始化成功，开始启动消息读取协程")

	p.isStreaming = true

	// 启动读取消息的协程
	go func() {
		p.ReadMessage(ctx)
	}()

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
	_ = resp
	//respStruct := ParseResponse(resp)
	return nil
}

func (p *Provider) sendMessages(segmentSize int, content []byte, stopChan <-chan struct{}) error {
	messageChan := make(chan []byte)
	go func() {
		for message := range messageChan {
			err := p.conn.WriteMessage(websocket.TextMessage, message)
			if err != nil {
				//log.Printf("write message err: %s", err)
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
			//log.Printf("send message: seq: %d", p.seq)
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
			//log.Fatalf("failed to send audio stream: %s", err)
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
