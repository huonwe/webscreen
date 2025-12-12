package scrcpy

type DataReceiver struct {
	VideoChan chan []byte
	AudioChan chan []byte
}

func (r *DataReceiver) New() *DataReceiver {
	return &DataReceiver{
		VideoChan: make(chan []byte, 100),
		AudioChan: make(chan []byte, 100),
	}
}

func (r *DataReceiver) Close() {
	close(r.VideoChan)
	close(r.AudioChan)
}

func (r *DataReceiver) ReceiveVideoFrame(frame []byte) {
	r.VideoChan <- frame
}

func (r *DataReceiver) ReceiveAudioFrame(frame []byte) {
	r.AudioChan <- frame
}
