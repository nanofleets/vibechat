import Fingerprint2 from 'fingerprintjs2'
import { useEffect, useRef, useState } from 'react'
import './App.css'

// Always use same domain for API (relative path)
const API_URL = ''

// Construct WebSocket URL based on current protocol and host
const WS_URL = `${window.location.protocol === 'https:' ? 'wss:' : 'ws:'}//${window.location.host}`

function App() {
  const [nick, setNick] = useState('')
  const [fingerprint, setFingerprint] = useState('')
  const [hasNick, setHasNick] = useState(false)
  const [messages, setMessages] = useState([])
  const [users, setUsers] = useState([])
  const [inputMessage, setInputMessage] = useState('')
  const [wsStatus, setWsStatus] = useState('disconnected')
  const [hoveredUser, setHoveredUser] = useState(null)
  
  const wsRef = useRef(null)
  const messagesEndRef = useRef(null)

  useEffect(() => {
    // Generate fingerprint
    Fingerprint2.get((components) => {
      const values = components.map(component => component.value)
      const fp = Fingerprint2.x64hash128(values.join(''), 31)
      setFingerprint(fp)
    })
  }, [])

  useEffect(() => {
    if (hasNick && fingerprint) {
      connectWebSocket()
      loadMessages()
    }

    return () => {
      if (wsRef.current) {
        wsRef.current.close()
      }
    }
  }, [hasNick, fingerprint])

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [messages])

  const loadMessages = async () => {
    try {
      const response = await fetch(`${API_URL}/api/chat/messages`)
      const data = await response.json()
      setMessages(data || [])
    } catch (error) {
      console.error('Failed to load messages:', error)
    }
  }

  const connectWebSocket = () => {
    const ws = new WebSocket(`${WS_URL}/api/ws?nick=${encodeURIComponent(nick)}&fingerprint=${encodeURIComponent(fingerprint)}`)
    
    ws.onopen = () => {
      setWsStatus('connected')
      console.log('WebSocket connected')
    }

    ws.onclose = () => {
      setWsStatus('disconnected')
      console.log('WebSocket disconnected')
      // Reconnect after 3 seconds
      setTimeout(() => {
        if (hasNick && fingerprint) {
          connectWebSocket()
        }
      }, 3000)
    }

    ws.onerror = (error) => {
      console.error('WebSocket error:', error)
      setWsStatus('error')
    }

    ws.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data)
        if (data.type === 'message') {
          setMessages(prev => [...prev, data.message])
        } else if (data.type === 'userlist') {
          // Sort users by nick to keep list stable
          const sortedUsers = (data.users || []).sort((a, b) => 
            a.nick.localeCompare(b.nick)
          )
          setUsers(sortedUsers)
        }
      } catch (error) {
        console.error('Failed to parse message:', error)
      }
    }

    wsRef.current = ws
  }

  const handleSetNick = (e) => {
    e.preventDefault()
    if (nick.trim()) {
      setHasNick(true)
    }
  }

  const handleSendMessage = async (e) => {
    e.preventDefault()
    if (!inputMessage.trim()) return

    try {
      await fetch(`${API_URL}/api/chat/messages`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          nick,
          text: inputMessage,
          fingerprint,
        }),
      })
      setInputMessage('')
    } catch (error) {
      console.error('Failed to send message:', error)
    }
  }

  const getStatusColor = () => {
    switch (wsStatus) {
      case 'connected': return '#00ff00'
      case 'disconnected': return '#ff0000'
      case 'error': return '#ffaa00'
      default: return '#888888'
    }
  }

  if (!hasNick) {
    return (
      <div className="nick-container">
        <div className="nick-prompt">
          <h1>vibechat</h1>
          <p>Choose your nickname to join the chat</p>
          <form onSubmit={handleSetNick}>
            <input
              type="text"
              value={nick}
              onChange={(e) => setNick(e.target.value)}
              placeholder="Enter your nickname..."
              maxLength={20}
              autoFocus
            />
            <button type="submit">Join Chat</button>
          </form>
        </div>
      </div>
    )
  }

  return (
    <div className="app">
      <header className="header">
        <span className="app-title">[vibechat]</span>
        <span className="arrow">→</span>
        <span className="nick-display">{nick}</span>
        <span className="status" style={{ color: getStatusColor() }}>
          [{wsStatus}]
        </span>
      </header>

      <div className="main-content">
        <div className="sidebar">
          <h3>Users ({users.length})</h3>
          <div className="users-list">
            {users.map((user, idx) => (
              <div
                key={idx}
                className="user-item"
                onMouseEnter={() => setHoveredUser(user)}
                onMouseLeave={() => setHoveredUser(null)}
                title={user.fingerprint}
              >
                <div className="user-info">
                  <span className="user-nick">{user.nick}</span>
                  {user.tagline && (
                    <span className="user-tagline">{user.tagline}</span>
                  )}
                </div>
                {hoveredUser?.nick === user.nick && (
                  <span className="user-fingerprint">
                    {user.fingerprint.substring(0, 8)}...
                  </span>
                )}
              </div>
            ))}
          </div>
        </div>

        <div className="chat-container">
          <div className="messages">
            {messages.map((msg, idx) => (
              <div key={idx} className="message">
                <span className="message-nick">{msg.nick}:</span>
                <span className="message-text">{msg.text}</span>
                <span className="message-time">
                  {new Date(msg.timestamp).toLocaleTimeString()}
                </span>
              </div>
            ))}
            <div ref={messagesEndRef} />
          </div>

          <form className="input-form" onSubmit={handleSendMessage}>
            <input
              type="text"
              value={inputMessage}
              onChange={(e) => setInputMessage(e.target.value)}
              placeholder="Type a message..."
              autoFocus
            />
            <button type="submit">Send</button>
          </form>
        </div>
      </div>
    </div>
  )
}

export default App
