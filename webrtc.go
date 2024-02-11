package main

import (
	"encoding/json"
	"log"
	"sync"

	"github.com/pion/rtcp"
	"github.com/pion/webrtc/v3"
)

type Peers struct {
	ListLock    sync.RWMutex // lock for what? I guess for the peers to do stuff, i.e. join/remove/etc
	Connections []PeerConnectionState
	TrackLocals map[string]*webrtc.TrackLocalStaticRTP
	// track -> media, local -> camera, audio,
	// static -> fixed params like codec that don't change in lifetime of call
	// rtp -> protocol used for communication
}

func (p *Peers) AddTrack(t *webrtc.TrackRemote) *webrtc.TrackLocalStaticRTP {
	p.ListLock.Lock()
	defer func() {
		p.ListLock.Unlock()
		p.SignalPeerConnections()
	}()

	trackLocal, err := webrtc.NewTrackLocalStaticRTP(t.Codec().RTPCodecCapability, t.ID(), t.StreamID())
	if err != nil {
		log.Println(err.Error())
		return nil
	}

	p.TrackLocals[t.ID()] = trackLocal
	return trackLocal
}

func (p *Peers) RemoveTrack(t *webrtc.TrackLocalStaticRTP) {
	p.ListLock.Lock()
	defer func() {
		p.ListLock.Unlock()
		p.SignalPeerConnections()
	}()

	delete(p.TrackLocals, t.ID())
}

// responsible for sending Picture Loss Indication (PLI) RTCP packets to all connected
// peers for each available track to request a keyframe or signal picture loss.
func (p *Peers) DispatchKeyFrame() {
	p.ListLock.Lock()
	defer p.ListLock.Unlock()

	for i := range p.Connections {
		for _, reciever := range p.Connections[i].PeerConnection.GetReceivers() {
			if reciever.Track() == nil { // no audio video?
				continue
			}

			// rtp -> transport audio video
			// rtcp -> real time control protocol, monitor transmission
			_ = p.Connections[i].PeerConnection.WriteRTCP([]rtcp.Packet{
				&rtcp.PictureLossIndication{
					MediaSSRC: uint32(reciever.Track().SSRC()),
				},
			})
		}
	}
}

func (p *Peers) SignalPeerConnections() {
	p.ListLock.Lock()
	defer func() {
		p.ListLock.Unlock()
		p.DispatchKeyFrame()
	}()

	for syncAttempt := 0; syncAttempt < 25; syncAttempt++ {
		if !p.attemptSync() {
			break
		}
	}

	// if we reach this point then either the sync was successful or we have reached the attempt limit
}

func (p *Peers) attemptSync() bool {
	for i := range p.Connections {
		if p.removeClosedConnection(i) {
			return true // try again
		}

		if err := p.syncSendersAndReceivers(i); err != nil {
			return true // try again
		}

		if err := p.sendOffer(i); err != nil {
			return true // try again
		}
	}

	return false // synchronization successful
}

func (p *Peers) removeClosedConnection(index int) bool {
	if p.Connections[index].PeerConnection.ConnectionState() == webrtc.PeerConnectionStateClosed {
		// Remove closed connection
		p.Connections = append(p.Connections[:index], p.Connections[index+1:]...)
		// log.Println("Updated connections list:", p.Connections)
		return true // try again
	}
	return false
}

func (p *Peers) syncSendersAndReceivers(index int) error {
	existingSenders := map[string]bool{}

	for _, sender := range p.Connections[index].PeerConnection.GetSenders() {
		if sender.Track() == nil {
			continue
		}
		existingSenders[sender.Track().ID()] = true

		// Remove the track if it's not supposed to be sent
		if _, ok := p.TrackLocals[sender.Track().ID()]; !ok {
			if err := p.Connections[index].PeerConnection.RemoveTrack(sender); err != nil {
				return err
			}
		}
	}

	// Iterate over receivers and add them to senders
	for _, receiver := range p.Connections[index].PeerConnection.GetReceivers() {
		if receiver.Track() == nil {
			continue
		}
		existingSenders[receiver.Track().ID()] = true
	}

	// Add tracks that are in local but not in existing senders
	for trackID := range p.TrackLocals {
		if _, ok := existingSenders[trackID]; !ok {
			if _, err := p.Connections[index].PeerConnection.AddTrack(p.TrackLocals[trackID]); err != nil {
				return err
			}
		}
	}

	return nil
}

func (p *Peers) sendOffer(index int) error {
	offer, err := p.Connections[index].PeerConnection.CreateOffer(nil)
	if err != nil {
		return err
	}

	if err = p.Connections[index].PeerConnection.SetLocalDescription(offer); err != nil {
		return err
	}

	offerString, err := json.Marshal(offer)
	if err != nil {
		return err
	}

	if err = p.Connections[index].Websocket.WriteJSON(&websocketMessage{
		Event: "offer",
		Data:  string(offerString),
	}); err != nil {
		return err
	}

	return nil
}
