# Telephony Stack Learning Guide (Go-Focused)

This guide provides Go-specific learning resources and implementation guidance for the telephony stack used in production. The resources are organized by category with focus on what you'll actually touch in code, what you'll configure, and what will trip you up.

## Table of Contents
- [Core Telephony](#core-telephony)
- [Protocols](#protocols)
- [Stack Context](#stack-context)
- [Suggested Learning Order](#suggested-learning-order)

---

## Core Telephony

### 1. FreeSWITCH — Learn the boundaries first

#### ESL (Event Socket Library)
**Purpose**: Programmatic control of FreeSWITCH via TCP socket interface for real-time event handling and call control.

**What to learn:**
- **Inbound vs. outbound ESL connections**: Inbound: your Go service connects to FreeSWITCH. Outbound: FreeSWITCH connects to your Go service on a per-call basis. Know when to use each.
- **Dialplan XML**: You won't write much of it long-term, but you must understand how it hands off to your Go logic. Learn `socket` app for outbound ESL, and how to set channel variables that your Go service will read.
- **mod_sofia**: Understand SIP profiles (internal vs. external), how they bind to interfaces, and how ACLs work. Misconfigured profiles are the #1 cause of "calls work locally but not remotely."

**Go-specific:**
- Use a well-maintained ESL library—`github.com/percipia/eslgo` or write your own minimal one. The wire protocol is text-based and easy to parse, but connection lifecycle management matters.
- Design for reconnection. FreeSWITCH restarts or ESL disconnects will happen. Your Go service must handle this gracefully.

**What will bite you:**
- ESL's event model is firehose-by-default. Filter with `myevents` or you'll flood your Go service.
- Blocking in your ESL handler blocks the entire channel. Goroutines are your friend, but you must respect channel lifecycle—don't process a channel after it's hung up.

**Learning Resources:**
- [FreeSWITCH ESL Documentation](https://developer.signalwire.com/freeswitch/FreeSWITCH-Explained/Client-and-Developer-Interfaces/Event-Socket-Library/) - Official documentation with examples
- [ESL Example Clients](https://developer.signalwire.com/freeswitch/FreeSWITCH-Explained/Introduction/Event-System/ESL-Example-Clients_27591923/) - Working examples in multiple languages
- [Event Socket Outbound](https://developer.signalwire.com/freeswitch/FreeSWITCH-Explained/Client-and-Developer-Interfaces/Event-Socket-Library/Event-Socket-Outbound_3375460/) - Outbound connection patterns
- [percipia/eslgo](https://github.com/percipia/eslgo) - Go ESL library

#### Dialplan XML
**Purpose**: Call routing logic and application execution in FreeSWITCH.

**What to learn:**
- You won't write much of it long-term, but you must understand how it hands off to your Go logic
- Learn `socket` app for outbound ESL
- Learn how to set channel variables that your Go service will read

**Learning Resources:**
- [XML Dialplan Documentation](https://developer.signalwire.com/freeswitch/FreeSWITCH-Explained/Dialplan/XML-Dialplan/) - Complete XML dialplan reference
- [Dialplan Overview](https://developer.signalwire.com/freeswitch/FreeSWITCH-Explained/Dialplan/) - General dialplan concepts
- [Default Dialplan Example](https://github.com/signalwire/freeswitch/blob/master/conf/vanilla/dialplan/default.xml) - Production-grade example

#### mod_sofia
**Purpose**: SIP endpoint module for FreeSWITCH, handles SIP signaling and protocol implementation.

**What to learn:**
- Understand SIP profiles (internal vs. external), how they bind to interfaces, and how ACLs work
- Misconfigured profiles are the #1 cause of "calls work locally but not remotely"

**Learning Resources:**
- [mod_sofia Documentation](https://developer.signalwire.com/freeswitch/FreeSWITCH-Explained/Modules/mod_sofia_1048707/) - Module configuration and parameters
- [Sofia Configuration Files](https://developer.signalwire.com/freeswitch/FreeSWITCH-Explained/Configuration/Sofia-SIP-Stack/Sofia-Configuration-Files_7144453/) - SIP stack configuration
- [Sofia SIP Stack Guide](https://wdd.js.org/freeswitch/sofia-stack/) - Comprehensive Sofia SIP documentation

---

### 2. Kamailio — Routing brain, not media

#### Routing Logic
**Purpose**: SIP proxy routing and load balancing for high-volume traffic.

**What to learn:**
- Understand the request-route block, how to match on method, and how `$rU`, `$fu`, `$tu` work
- You'll use Kamailio to decide which FreeSWITCH instance or RTPengine box handles a call

**Go-specific:**
- Kamailio has a JSON-RPC interface. Your Go service can push dispatcher list updates, reload routing tables, or query registration status
- For dynamic routing decisions, consider having Kamailio do an HTTP lookup to your Go service using `http_async_client` module. Keep it fast—timeouts will block SIP signaling

**What will bite you:**
- SIP signaling is synchronous in Kamailio. External queries (HTTP, DB) add latency. Always use `async` versions

**Learning Resources:**
- [DISPATCHER Module Documentation](https://www.kamailio.org/docs/modules/devel/modules/dispatcher.html) - Latest dispatcher module docs
- [Kamailio as Load Balancer Guide](https://www.sinologic.net/en/2026-03/kamailio-as-a-load-balancer-for-asterisk-a-practical-guide-with-the-dispatcher-module.html) - Practical tutorial
- [Dispatcher Module Hidden Gem](https://www.kwancro.com/post/kamailio-dispatcher-module-hidden-gem/) - Advanced techniques
- [Dispatcher Configuration Example](https://github.com/kamailio/kamailio/blob/master/src/modules/dispatcher/doc/dispatcher.cfg) - Sample configuration

#### Dispatcher Module
**Purpose**: Load balancing to multiple FreeSWITCH instances.

**What to learn:**
- This is how you load-balance to multiple FreeSWITCH instances
- Learn the `ds_select_dst` algorithm options. Round-robin is fine to start; hash-based stickiness matters for registrations

**Learning Resources:**
- [DISPATCHER Module Documentation](https://www.kamailio.org/docs/modules/devel/modules/dispatcher.html) - Latest dispatcher module docs
- [DISPATCHER Module 4.3.x](https://kamailio.org/docs/modules/4.3.x/modules/dispatcher.html) - Stable version docs

#### Presence Handling
**Purpose**: SIP SIMPLE presence server for user availability and status.

**What to learn:**
- Understand `pua` and `presence` modules if you're doing BLF or presence subscriptions
- This is a deep topic; scope it carefully

**What will bite you:**
- Presence is notoriously complex. If mobile clients register/unregister frequently, presence updates can hammer your system. Rate-limit and coalesce.

**Learning Resources:**
- [Presence Module Documentation](https://www.kamailio.org/docs/modules/5.2.x/modules/presence.html) - Latest presence module
- [Kamailio Presence Made Simple](https://kb.asipto.com/kamailio:presence:k31-made-simple) - Step-by-step tutorial
- [Presence Configuration Example](https://github.com/kamailio/kamailio/blob/master/test/unit/presence.cfg) - Unit test configuration

---

### 3. RTPengine — Media proxy and codec wizard

#### Control Protocol
**Purpose**: Media relay, transcoding, and NAT traversal for RTP streams.

**What to learn:**
- RTPengine uses a UDP control protocol (ng protocol). You send `offer`/`answer` commands from your Go code, and it returns SDP with its own IP:port inserted. Learn the packet structure—it's simple string-based framing.
- ICE/Lite and WebRTC bridging: This is where RTPengine shines. It does ICE termination, DTLS-SRTP, and NAT traversal for legacy SIP endpoints talking to WebRTC clients.

**Go-specific:**
- Build or use a lightweight ng protocol client. The format is `command cookie md5hash\n` plus parameters. It's straightforward enough to write a clean Go implementation.
- Handle asynchronous responses. RTPengine can send back responses out of order; your Go code must match cookies.
- RTPengine can run in pairs (active/active or active/passive). Your Go service should track which instance holds a call and send `delete` to the right one.

**What will bite you:**
- Forgetting to `delete` sessions will leak ports and eventually exhaust the proxy.
- If your Go service sends `answer` before receiving the `offer` response, you'll get errors. Implement proper sequencing.
- SRTP keying for WebRTC requires SDES or DTLS fingerprint exchange. Understand which your setup uses.

#### Codec Negotiation
**Purpose**: Media codec handling and transcoding.

**What to learn:**
- RTPengine can transcode, but you should prefer pass-through. Understand how `codec-mask`, `codec-strip`, and `codec-transcode` directives affect the SDP.
- Your Go logic should decide when to allow transcoding vs. reject a call.

**Learning Resources:**
- [RTPengine GitHub Repository](https://github.com/sipwise/rtpengine) - Official source code and docs
- [RTPengine Manual](https://rtpengine.readthedocs.io/en/latest/rtpengine.html) - Complete manual
- [Transcoding Documentation](https://github.com/sipwise/rtpengine/blob/master/docs/transcoding.md) - Codec transcoding guide

---

### 4. SIP Fundamentals — The call flow you'll debug at 2 AM

#### INVITE/BYE Flow
**Purpose**: Understanding SIP session establishment and teardown.

**What to learn:**
- Internalize the full ladder diagram including provisional responses (100 Trying, 180 Ringing, 183 Session Progress), PRACK if using 100rel, and the ACK three-way handshake. Your Go logic will need to track call state.

**Go-specific:**
- Build a finite state machine for calls. SIP dialogs have defined states (early, confirmed, terminated). Your CDR and billing logic depend on accurate state tracking.

**What will bite you:**
- Re-INVITEs mid-call (for hold, codec change). If your Go logic doesn't handle them, calls will break silently.
- Early media before 200 OK. Some carriers send media during 183; your RTPengine setup must handle this.
- Transient 408 Timeout vs. definitive 486 Busy. Error handling must distinguish recoverable from final failures.

**Learning Resources:**
- [SIP Call Flow Explained Step by Step](https://www.thevoco.com/blog/sip-call-flow-explained-step-by-step-invite-bye) - Detailed call flow tutorial
- [SIP Protocol Call Flow Guide](https://videosdk.live/developer-hub/sip/sip-protocol-call-flow) - In-depth protocol guide
- [SIP INVITE Explained](https://www.thevoco.com/blog/sip-invite-explained-with-call-flow) - INVITE-specific flow

#### SDP Negotiation
**Purpose**: Media capability exchange and session description.

**What to learn:**
- Understand how to parse SDP, extract codec lists, IP addresses, and media directions (sendrecv/sendonly/recvonly/inactive). This is critical for bridging scenarios.

**Go-specific:**
- Parse SIP messages carefully. Use a library that handles line folding, multi-value headers, and parameterized URIs. `github.com/emiago/sipgox` or `siprocket` are options, but validate them against your needs.

**Learning Resources:**
- [SIP & SDP Explained: Complete Practical Guide](https://www.thevoco.com/blog/sip-sdp-explained-the-complete-practical-guide-with-examples) - Comprehensive SDP guide
- [SIP Offer/Answer Model](https://www.tutorialspoint.com/session_initiation_protocol/session_initiation_protocol_the_offer-answer_model.htm) - RFC 3264 implementation

#### NAT Traversal
**Purpose**: Handling SIP signaling and media through NAT devices.

**What to learn:**
- Learn the difference between STUN, TURN, and SBC-style media anchoring. RTPengine acts as the anchor, but your Go service must ensure both parties use the anchored path.
- Also understand SIP ALG and why it's the enemy.

**Learning Resources:**
- [SIP Call Flow - NAT Traversal Section](https://www.thevoco.com/blog/sip-call-flow-explained-step-by-step-invite-bye) - NAT traversal in call flow
- [SIP & SDP Guide - NAT Section](https://www.thevoco.com/blog/sip-sdp-explained-the-complete-practical-guide-with-examples) - STUN/TURN solutions

---

## Protocols

### 5. Protocols — The transport layer truth

#### SIP over UDP, TCP, and TLS
**Purpose**: Transport layer options for SIP signaling with different security and reliability characteristics.

**What to learn:**
- Understand TCP state tracking, keep-alives, and TLS certificate verification
- Mobile clients switching networks mid-call will stress your TCP reconnection logic

**Go-specific:**
- For SRTP key management, understand the SDES exchange: Crypto header in SDP, parsed to extract the key-salt and crypto suite
- WebRTC signaling is usually JSON over WebSocket. Your Go service will translate between this and SIP over ESL/Kamailio. Build clean domain types that model both sides.

**Learning Resources:**
- [SIP Security Protocols: UDP vs TCP vs TLS](https://www.avoxi.com/blog/sip-security-protocols-udp-tcp-tls/) - Security comparison
- [SIP over TLS Documentation](https://www.myvoipapp.com/docs/mss_services/sip-over-tls/index.html) - TLS implementation guide
- [AVOXI SIP Protocol Guide](https://support.avoxi.com/system-and-network-best-practices/sip-protocol-udp-vs-tcp) - Transport selection
- [TCP vs UDP Guide](https://voximplant.com/blog/tcp_vs_udp) - Transport protocol comparison

#### RTP and SRTP Media Flow
**Purpose**: Real-time media transport with optional encryption.

**What to learn:**
- Learn the packet structure (sequence number, timestamp, SSRC)
- You don't need to process RTP yourself unless building monitoring, but you must understand SSRC rotation and how SRTP key derivation works

**Learning Resources:**
- [uvgRTP Library](https://github.com/ultravideo/uvgRTP) - Open-source RTP/SRTP implementation
- [Cloudinary RTP Guide](https://cloudinary.com/guides/live-streaming-video/real-time-protocol) - RTP fundamentals
- [Wikipedia RTP](https://en.wikipedia.org/wiki/Real-Time_Transport_Protocol) - Protocol specification
- [GetStream RTP Glossary](https://getstream.io/glossary/real-time-transport-protocol-rtp/) - RTP vs RTCP vs SRTP

#### WebRTC to SIP Bridging
**Purpose**: Connecting web browsers to traditional SIP networks.

**What to learn:**
- This is the hard part. WebRTC mandates ICE, DTLS-SRTP, and trickle ICE candidates. RTPengine handles most, but your Go service must manage the offer/answer exchange and ensure codec agreement ends in something both sides support.

**Go-specific:**
- WebRTC signaling is usually JSON over WebSocket. Your Go service will translate between this and SIP over ESL/Kamailio. Build clean domain types that model both sides.

**Learning Resources:**
- [WebRTC to SIP GitHub](https://github.com/havfo/WEBRTC-to-SIP) - Complete setup guide
- [WebRTC to SIP Gateway Tutorial](https://www.mizu-voip.com/Software/WebRTCtoSIP.aspx) - Gateway implementation
- [Janus SIP Gateway Plugin](https://webrtc.ventures/2017/10/janus-webrtc-gateway-as-a-sip-gateway-how-to-monitor-it/) - Media server integration
- [WebRTC SIP Integration Advanced Techniques](https://webrtc.ventures/2025/07/webrtc-sip-integration-advanced-techniques-for-real-time-web-and-telephony-communication/) - Advanced patterns
- [SIP.js Library](https://sipjs.com/) - JavaScript SIP client for WebRTC

---

## Stack Context

### 6. Multi-tenant CDR — Get this right early

**Purpose**: Recording call details across multiple tenants/domains with proper isolation and reporting.

**What to learn:**
- CDR structure: At minimum, capture A-leg and B-leg call UUIDs, tenant IDs, timestamps, durations, SIP hangup causes, and hold time
- Multi-tenancy: Every call must carry a tenant identifier through the entire stack. Channel variables in FreeSWITCH, custom SIP headers (e.g., X-Tenant-ID) between Kamailio and FreeSWITCH, and tracking in your Go service's CDR database

**Go-specific:**
- Capture CDR events from FreeSWITCH ESL (`CHANNEL_HANGUP_COMPLETE`), but also generate your own enriched CDR with metadata from Kamailio signaling context and RTP stats
- Consider exactly-once delivery semantics. ESL can re-deliver events on reconnection. Deduplicate by call UUID and timestamp
- Store first, process later. Insert raw CDR immediately, then enrich asynchronously

**Learning Resources:**
- [Complete CDR Guide](https://www.3cx.com/docs/call-detail-record-guide/) - CDR field definitions
- [CDR Usage Guide](https://www.3cx.com/docs/cdr-call-data-records/) - CDR configuration and output
- [Multi-tenant CDR Best Practices](https://www.pbxforums.com/threads/best-practice-to-extract-cdr-report-for-a-tenant-in-multi-tenant-setup.5299/) - Tenant isolation strategies
- [Telnyx CDR Understanding](https://support.telnyx.com/en/articles/1130662-understanding-telnyx-cdr) - Cloud provider CDR structure

---

### 7. Answering Machine Detection (AMD) — Tradeoffs

**Purpose**: Distinguishing between human and machine answers to optimize call handling.

**What to learn:**
- FreeSWITCH has built-in AMD, but it's audio-based and imperfect. Understand the parameters: `initial_silence`, `greeting`, `after_greeting_silence`, `total_analysis_time`
- Modern approaches use ML classifiers on audio. This is a pipeline design problem: capture early media stream, feed to classifier, return verdict before the call is bridged

**Go-specific:**
- If you're building a custom AMD pipeline, your Go service will likely stream early audio to a classification service. Consider the latency budget—you have 2-3 seconds typically
- AMD result must be consumed by dialplan routing (voicemail vs. connect) and recorded in CDR

**Learning Resources:**
- [Twilio AMD Documentation](https://www.twilio.com/docs/voice/answering-machine-detection) - Cloud AMD implementation
- [AI-based AMD for VICIdial](https://medium.com/@akhanriz/ai-based-answering-machine-detection-for-vicidial-freeswitch-and-asterisk-cb73320e7f28) - ML approach to AMD
- [ML AMD for VICIdial](https://www.vicidial.org/VICIDIALforum/viewtopic.php?t=42330) - Community ML project
- [Talkdesk AMD Overview](https://support.talkdesk.com/hc/en-us/articles/9865745786267-Answering-Machine-Detection-AMD-Overview) - Enterprise AMD features

---

### 8. WireGuard VPN — Carrier bypass via zero-rating

**Purpose**: Secure tunneling for VoIP traffic with zero-rating (no data charges) on carrier networks.

**What to learn:**
- WireGuard is kernel-level and extremely efficient. Understand how to configure peer endpoints and allowed IPs on both the client (mobile) and server side
- Zero-rating: If mobile operators don't count WireGuard traffic against data caps, all media flows through the VPN tunnel. This means your SIP signaling should also go through it, or at minimum, the SDP must advertise the VPN-side IPs

**Go-specific:**
- Use `golang.zx2c4.com/wireguard/wgctrl` to manage WireGuard interfaces and peers programmatically from your control plane
- Design for peer lifecycle: provision a WireGuard peer when a user enables zero-rated calling, tear it down when they disconnect. Keep peer tables clean

**Learning Resources:**
- [WireGuard Quick Start](https://www.wireguard.com/quickstart/) - Official quickstart guide
- [Zero-Trust WireGuard VPN](https://medium.com/codex/breaking-down-the-castle-walls-building-a-zero-trust-wireguard-vpn-that-actually-works-fb257de775e9) - Zero-trust implementation
- [WireGuard Guide GitHub](https://github.com/mikeroyal/WireGuard-Guide) - Comprehensive WireGuard guide

---

### 9. FCM Push Notifications — iOS call reliability

**Purpose**: Waking suspended SIP clients for incoming calls using Firebase Cloud Messaging.

**What to learn:**
- Android has FCM; iOS uses APNs but can relay through FCM. Understand the payload format: priority must be `high` for VoIP wake-ups, and the notification must contain SIP INVITE metadata (caller, call ID) so the app can register and receive the call
- FCM delivery is not guaranteed. Your signaling server should re-push after a timeout if the client hasn't registered within X seconds

**Go-specific:**
- Use `firebase.google.com/go/v4/messaging` for FCM integration
- Implement push/re-registration timeout tracking. If the client doesn't come online within 8-10 seconds, likely the call won't connect; route to voicemail and cancel further pushes
- Handle the iOS VoIP push deprecation: newer iOS versions require the app to use CallKit and standard push, which has different timing behavior. Test this end-to-end

**Learning Resources:**
- [Siprix Push Notifications](https://docs.siprix-voip.com/rst/pushnotif.html) - Android SIP push implementation
- [PortSIP Push Notifications](https://support.portsip.com/development-portsip/mobile-push-notifications/implement-push-notifications-in-android-app-with-portsip-pbx) - iOS/Android integration
- [FCM for SIP Clients (StackOverflow)](https://stackoverflow.com/questions/69564617/send-fcm-push-notification-to-sip-client-app-on-android-based-on-pn-param-and-pn) - RFC 8599 implementation
- [VoIP Push Notifications PDF](https://www.mizu-voip.com/Portals/0/Files/VOIP_Push_notifications.pdf) - Comprehensive guide
- [RFC 8599 - SIP Push](https://datatracker.ietf.org/doc/rfc8599/) - Official specification

---

## Suggested Learning Order

1. **SIP fundamentals + FreeSWITCH ESL** (you need to see calls end to end first)
2. **Kamailio routing basics + dispatcher** (load balancing before scale matters)
3. **RTPengine control protocol** (media anchoring)
4. **WebRTC bridging** (hardest bit—tackle after core is solid)
5. **CDR, AMD, WireGuard, FCM** (features on top of the core)

---

## Additional Resources

### Communities and Forums
- [FreeSWITCH Community](https://freeswitch.org/confluence/) - Official documentation and forums
- [Kamailio mailing lists](https://lists.kamailio.org/cgi-bin/mailman/listinfo) - Community support
- [VoIP-Info Forums](https://www.voip-info.org/) - General VoIP discussions
- [Stack Overflow - VoIP tag](https://stackoverflow.com/questions/tagged/voip) - Q&A platform

### Testing Tools
- [SIPp](http://sipp.sourceforge.net/) - SIP traffic generator
- [Wireshark](https://www.wireshark.org/) - Protocol analyzer
- [sipsak](https://sipsak.org/) - SIP testing tool
- [Jitsi](https://jitsi.org/) - WebRTC testing client

---

## Conclusion

This Go-focused learning guide provides practical, implementation-specific guidance for mastering the telephony stack. The key to success is consistent practice and building real systems. Don't just read the documentation—implement each concept in a test environment and iterate based on real-world testing.

The focus on Go-specific implementation details, common pitfalls ("what will bite you"), and the suggested learning order will help you avoid the most common mistakes and build production-ready telephony solutions efficiently.
