package gosherpa

import (
	"context"
	"time"
	"xiaozhi-server-go/src/core/providers/asr"

	"github.com/gorilla/websocket"
)

type Provider struct {
	*asr.BaseProvider
	conn *websocket.Conn
}

func NewProvider(config *asr.Config, deleteFile bool) (*Provider, error) {
	base := asr.NewBaseProvider(config, deleteFile)

	provider := &Provider{
		BaseProvider: base,
	}
	// 初始化音频处理
	provider.InitAudioProcessing()
	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second, // 设置握手超时
	}
	conn, _, err := dialer.DialContext(context.Background(), config.Data["addr"].(string), map[string][]string{})
	if err != nil {
		return nil, err
	}
	provider.conn = conn
	go func() {
		defer func() {
			if err := recover(); err != nil {
			}
		}()
		for {
			messageType, p, _ := conn.ReadMessage()
			if messageType == websocket.TextMessage {
				if listener := provider.GetListener(); listener != nil {
					if finished := listener.OnAsrResult(string(p)); finished {

					}
				}
			}
		}

	}()

	return provider, nil
}

func (p *Provider) Transcribe(ctx context.Context, audioData []byte) (string, error) {
	return "", nil
}

// 添加音频数据到缓冲区
func (p *Provider) AddAudio(data []byte) error {
	p.conn.WriteMessage(websocket.BinaryMessage, data)

	return nil
}

// 复位ASR状态
func (p *Provider) Reset() error {
	return nil
}

func init() {
	asr.Register("gosherpa", func(config *asr.Config, deleteFile bool) (asr.Provider, error) {
		return NewProvider(config, deleteFile)
	})
}
