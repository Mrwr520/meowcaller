# meowcaller API Reference
> Source: https://pkg.go.dev/github.com/purpshell/meowcaller

## Constants

```go
const (
    SampleRate   = 16000
    FrameSamples = 960
)
```
Audio is 16 kHz mono float32 PCM, 60 ms frames. All AudioSource/AudioSink use this format.

---

## Types Overview

### Client
Entry point. Wraps `*whatsmeow.Client`.

```go
func NewClient(wa *whatsmeow.Client, opts ...Option) *Client

func (c *Client) Call(ctx, target string) (*Call, error)
func (c *Client) CallWithOptions(ctx, target string, opts CallOptions) (*Call, error)
func (c *Client) OnIncomingCall(fn func(*Call))
func (c *Client) GroupCall(ctx, targets ...string) (*Call, error)
func (c *Client) GroupCallByID(ctx, groupID string) (*Call, error)
func (c *Client) JoinCallLink(ctx, tokenOrURL string, opts CallLinkOptions) (*Call, error)
func (c *Client) CreateCallLink(ctx, opts CallLinkOptions) (CallLink, error)
func (c *Client) PreviewCallLink(ctx, tokenOrURL string, opts CallLinkOptions) (CallLinkPreview, error)
```

> Construct before `whatsmeow.Connect()` so the raw call adapter is installed first.

---

### Call
One live direct or group call. All methods concurrency-safe.

#### Lifecycle / Signaling
```go
func (c *Call) Answer() error       // accept inbound (preaccept + accept + media up)
func (c *Call) Reject() error       // decline inbound
func (c *Call) Hangup() error       // end call either direction
func (c *Call) ID() string          // 32-char uppercase hex call-id
func (c *Call) State() CallPhase
func (c *Call) Peer() types.JID
```

#### Phase callbacks
```go
func (c *Call) OnStateChange(fn func(CallPhase))
func (c *Call) OnPeerAccept(fn func())     // outgoing: remote answered
func (c *Call) OnReady(fn func())          // relay bound, first frames exchanged
func (c *Call) OnEnd(fn func(reason string))
func (c *Call) OnMuteState(fn func(muted bool))  // remote mute_v2 state; true=muted
```

#### Audio
```go
func (c *Call) Play(src AudioSource) *Player   // shortcut: create+subscribe+start
func (c *Call) Subscribe(p *Player)             // attach player (replaces previous)
func (c *Call) Receive(sink AudioSink)          // attach inbound audio sink
```
> When no player is active (or player is idle), **silence is sent automatically** to keep the relay bridge alive.

#### Video
```go
func (c *Call) StartVideo() error
func (c *Call) StopVideo() error
func (c *Call) AcceptVideo() error
func (c *Call) SetVideoEnabled(enabled bool) error
func (c *Call) SetVideoOrientation(orientation int) error
func (c *Call) SendVideo(accessUnit []byte) error          // Annex-B H.264
func (c *Call) SendVideoWithDuration(accessUnit []byte, duration time.Duration) error
func (c *Call) ReceiveVideo(sink VideoSink)
func (c *Call) IsVideo() bool
func (c *Call) IsSendingVideo() bool
func (c *Call) IsReceivingVideo() bool
func (c *Call) OnVideoState(fn func(VideoState))
func (c *Call) OnVideoKeyframeRequest(fn func())
func (c *Call) OnParticipantVideoFrame(fn func(ParticipantVideoFrame))
```
> Video send/recv paths are marked **NOT VALIDATED** in the upstream source.

#### Group call
```go
func (c *Call) AddParticipant(ctx, target string) error
func (c *Call) AddParticipants(ctx, targets ...string) []error
func (c *Call) RingParticipant(ctx, target string) error
func (c *Call) GroupState() (GroupCallState, bool)
func (c *Call) OnGroupState(fn func(GroupCallState))
func (c *Call) SetHandRaised(raised bool) error
func (c *Call) OnHandRaise(fn func(HandRaiseState))
func (c *Call) SendReaction(emoji string) error
func (c *Call) OnReaction(fn func(CallReaction))
func (c *Call) OnScreenShare(fn func(ScreenShareState))
func (c *Call) StartScreenShare(screenShareID *uint32) error
func (c *Call) StopScreenShare() error
func (c *Call) ScreenShares() []ScreenShareState
```

#### Waiting room (call-link)
```go
func (c *Call) AdmitParticipant(ctx, user string) error
func (c *Call) DenyParticipant(ctx, user string) error
func (c *Call) SetApprovalRequired(ctx, enabled bool) error
func (c *Call) WaitingRoomState() (WaitingRoomState, bool)
func (c *Call) OnWaitingRoomState(fn func(WaitingRoomState))
```

---

### CallPhase
```go
const (
    CallPhaseIdle        CallPhase = iota
    CallPhaseCalling
    CallPhaseRinging
    CallPhaseConnecting
    CallPhaseActive
    CallPhaseEnded
    CallPhaseWaitingRoom
)
```

---

### CallOptions / GroupCallOptions
```go
type CallOptions struct {
    Video bool
}
type GroupCallOptions struct {
    GroupJID string
    Video    bool
}
```

---

### AudioSource
Yields 16 kHz mono PCM frames. Built-ins:

```go
func WAVFile(path string) (AudioSource, error)
func MP3File(path string) (AudioSource, error)
func OpusFile(path string) (AudioSource, error)
func PCMStream(r io.ReadCloser) AudioSource   // raw s16le mono 16kHz
```

---

### AudioSink
Consumes 16 kHz mono PCM frames from peer.

```go
func WAVRecorder(path string) (AudioSink, error)  // record to WAV

// Inline callback:
type SinkFunc func(frame []float32)
func (f SinkFunc) WriteFrame(frame []float32) error
func (f SinkFunc) Close() error
```

---

### Player
Streams an AudioSource into a call. Analog of discord.js AudioPlayer.

```go
func NewPlayer() *Player

func (p *Player) Play(src AudioSource)
func (p *Player) Pause()
func (p *Player) Resume()
func (p *Player) Stop()
func (p *Player) State() PlayerState
func (p *Player) OnFinish(fn func())   // source exhausted → PlayerIdle
```

```go
const (
    PlayerIdle    PlayerState = iota
    PlayerPlaying
    PlayerPaused
)
```

---

### VideoSink
```go
type VideoSink interface {
    WriteVideo(accessUnit []byte) error
    Close() error
}
func AnnexBRecorder(path string) (VideoSink, error)

type VideoSinkFunc func(accessUnit []byte)
```

---

### MediaPipeline
Low-level E2E SRTP. Used internally; exposed for advanced use.

```go
func NewMediaPipeline(callKey []byte, selfJID, peerJID string, ssrc, samplesPerPacket uint32, ...) (*MediaPipeline, error)

func (p *MediaPipeline) ProtectAudio(opusPayload []byte) ([]byte, error)
func (p *MediaPipeline) UnprotectAudio(packet []byte) (rtp.RtpHeader, []byte, bool)
func (p *MediaPipeline) ProtectRTP(header *rtp.RtpHeader, payload []byte) ([]byte, error)
func (p *MediaPipeline) RekeyRecv(callKey []byte, peerJID string) error
func (p *MediaPipeline) RekeyRecvFromRaw(rawE2E []byte, peerJID string) error
func (p *MediaPipeline) RekeySendFromRaw(rawE2E []byte, selfJID string) error
func (p *MediaPipeline) SenderStats() rtp.RtcpSenderStats
```

---

### CallRegistry
Thread-safe map of active calls.

```go
func NewCallRegistry() *CallRegistry

func (r *CallRegistry) Insert(session *CallSession) bool
func (r *CallRegistry) Remove(callID string) bool
func (r *CallRegistry) Phase(callID string) (CallPhase, bool)
func (r *CallRegistry) Transition(callID string, next CallPhase) bool
func (r *CallRegistry) SetMediaTask(callID string, cancel context.CancelFunc)
func (r *CallRegistry) AbortAll() int    // call on disconnect/reconnect
func (r *CallRegistry) ActiveCount() int
func (r *CallRegistry) Snapshot(callID string) (CallSession, bool)
```

---

### CallSession
Per-call signaling state with validated phase transitions.

```go
func NewOutgoingSession(callID string, peerJID, callCreator types.JID, opts ...Option) *CallSession
func NewIncomingSession(callID string, peerJID, callCreator types.JID, opts ...Option) *CallSession

func (s *CallSession) Phase() CallPhase
func (s *CallSession) TransitionTo(next CallPhase) bool
func (s *CallSession) IsActive() bool
func (s *CallSession) IsEnded() bool
```

---

### Options
```go
func WithLogger(l zerolog.Logger) Option        // surface debug/trace
func WithDiagnostics(rec *diag.Recorder) Option // JSONL dump of raw secrets+media — dev only
```

---

### AudioCodec
```go
const (
    AudioCodecMlow AudioCodec = iota
    AudioCodecOpus
)
func (c AudioCodec) String() string
```

---

### Other types
```go
type CallDirection  // CallDirectionOutgoing / CallDirectionIncoming
type VideoState     // Active bool, Upgrade bool, Orientation int, Raw int
type HandRaiseState // Participant types.JID, Raised bool
type GroupCallState // TransactionID uint32, RekeyRequested bool, Participants []GroupCallParticipant
type CallReaction   // Emoji string, Removed bool, ...
type ScreenShareState
type WaitingRoomState  // Enabled, IsAdmin, InWaitingRoom bool, Users []WaitingRoomUser
type ParticipantVideoFrame
type VideoOrientationSink interface { SetOrientation(orientation int) }
```

---

## Minimal usage example

```go
// 1. Setup
mc := meowcaller.NewClient(waClient, meowcaller.WithLogger(logger))

// 2. Outbound call
call, err := mc.Call(ctx, "+8613800000000")

// 3. Play audio into call
call.Play(meowcaller.WAVFile("greeting.wav"))

// 4. Receive peer audio
call.Receive(meowcaller.SinkFunc(func(frame []float32) {
    // 16kHz mono PCM frame (960 samples = 60ms)
}))

// 5. Lifecycle
call.OnReady(func() { log.Println("media flowing") })
call.OnEnd(func(reason string) { log.Println("ended:", reason) })

// 6. Inbound
mc.OnIncomingCall(func(call *meowcaller.Call) {
    call.OnReady(func() { /* attach sinks */ })
    call.Answer()
})
```
