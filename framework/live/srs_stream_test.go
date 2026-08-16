package live

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	apperr "github.com/Connorig/go-blackbox/component/error"
)

// TestListStreamsSRS5TopLevelVideoAudio SRS 5.x 顶层 video/audio 解析:
// 嵌套结构与平铺兼容字段同时填充。
func TestListStreamsSRS5TopLevelVideoAudio(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/streams/" {
			t.Errorf("path = %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"code": 0,
			"streams": [{
				"id": "vid-5617g86",
				"name": "test",
				"app": "live",
				"vhost": "vid-vq19967",
				"url": "/live/test",
				"publish": {"cid": "fc47p6e7", "active": true},
				"video": {"codec": "H264", "profile": "High", "level": "3.1", "width": 1280, "height": 720},
				"audio": {"codec": "AAC", "sample_rate": 44100, "channel": 2, "profile": "LC"}
			}]
		}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, 0)
	streams, err := client.ListStreams(context.Background())
	if err != nil {
		t.Fatalf("ListStreams: %v", err)
	}
	if len(streams) != 1 {
		t.Fatalf("len = %d", len(streams))
	}
	s := streams[0]
	if s.PublishCID != "fc47p6e7" {
		t.Errorf("PublishCID = %q", s.PublishCID)
	}
	if s.Video == nil {
		t.Fatal("Video must not be nil")
	}
	if s.Video.Codec != "H264" || s.Video.Profile != "High" || s.Video.Level != "3.1" || s.Video.Width != 1280 || s.Video.Height != 720 {
		t.Errorf("Video = %+v", s.Video)
	}
	if s.Audio == nil {
		t.Fatal("Audio must not be nil")
	}
	if s.Audio.Codec != "AAC" || s.Audio.SampleRate != 44100 || s.Audio.Channel != 2 || s.Audio.Profile != "LC" {
		t.Errorf("Audio = %+v", s.Audio)
	}
	// 平铺兼容字段同步填充
	if s.VideoCodec != "H264" || s.AudioCodec != "AAC" || s.Width != 1280 || s.Height != 720 {
		t.Errorf("flat fields: codec=%q/%q size=%dx%d", s.VideoCodec, s.AudioCodec, s.Width, s.Height)
	}
}

// TestListStreamsLegacyPublishVideoAudio 旧版 SRS publish.video/audio 兼容。
func TestListStreamsLegacyPublishVideoAudio(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{
			"code": 0,
			"streams": [{
				"id": "vid-1", "name": "legacy", "app": "live", "vhost": "v1", "url": "/live/legacy",
				"publish": {"cid": "cid-1", "video": {"codec": "H264", "width": 640, "height": 360}, "audio": {"codec": "AAC"}}
			}]
		}`))
	}))
	defer server.Close()

	streams, err := NewClient(server.URL, 0).ListStreams(context.Background())
	if err != nil {
		t.Fatalf("ListStreams: %v", err)
	}
	s := streams[0]
	if s.PublishCID != "cid-1" || s.VideoCodec != "H264" || s.AudioCodec != "AAC" || s.Width != 640 || s.Height != 360 {
		t.Errorf("legacy flat fields = %+v", s)
	}
	if s.Video == nil || s.Video.Codec != "H264" {
		t.Errorf("legacy Video = %+v", s.Video)
	}
}

// TestDenyMessageAppErr 业务错误 msg 不带错误码后缀,其他错误原样透传。
func TestDenyMessageAppErr(t *testing.T) {
	appErr := apperr.New("A0001", "invalid stream key")
	if got := denyMessage(appErr); got != "invalid stream key" {
		t.Errorf("denyMessage(apperr) = %q", got)
	}
	wrapped := apperr.Wrap(appErr, "A0001", "invalid stream key")
	if got := denyMessage(wrapped); got != "invalid stream key" {
		t.Errorf("denyMessage(wrapped) = %q", got)
	}
	plain := jsonRawError("boom")
	if got := denyMessage(plain); got != "boom" {
		t.Errorf("denyMessage(plain) = %q", got)
	}
	if got := denyMessage(nil); got != "denied" {
		t.Errorf("denyMessage(nil) = %q", got)
	}
}

// jsonRawError 普通 error 样本。
type jsonRawError string

func (e jsonRawError) Error() string { return string(e) }
