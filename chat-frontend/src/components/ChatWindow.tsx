import { useState, useEffect, useRef, useCallback } from 'react';
import type { Conversation, Message, WSServerEvent } from '../types/chat';
import { chatApi } from '../api/chat';
import { useWebSocket } from '../hooks/useWebSocket';
import { MessageBubble } from './MessageBubble';

interface Props {
  conversation: Conversation | null;
  currentUserId: number;
  token: string;
  onBack?: () => void;
}

export function ChatWindow({ conversation, currentUserId, token, onBack }: Props) {
  const [messages, setMessages] = useState<Message[]>([]);
  const [input, setInput] = useState('');
  const [loading, setLoading] = useState(false);
  const bottomRef = useRef<HTMLDivElement>(null);

  const handleWSEvent = useCallback((event: WSServerEvent) => {
    if (event.type === 'new_message') {
      const wsMsg = event.message;
      if (wsMsg.conversation_id === conversation?.ID) {
        const msg: Message = {
          ID: wsMsg.id,
          ConversationID: wsMsg.conversation_id,
          SenderID: wsMsg.sender_id,
          Content: wsMsg.content,
          ContentType: 'text',
          SentAt: wsMsg.sent_at,
          IsRead: false,
        };
        setMessages((prev) => {
          const exists = prev.some((m) => m.ID === msg.ID);
          return exists ? prev : [...prev, msg];
        });
      }
    }
  }, [conversation?.ID]);

  const { send } = useWebSocket(token, handleWSEvent);

  useEffect(() => {
    if (!conversation) return;
    setLoading(true);
    chatApi
      .getMessages(conversation.ID)
      .then(setMessages)
      .catch(console.error)
      .finally(() => setLoading(false));

    send({ type: 'mark_read', conversation_id: conversation.ID });
  }, [conversation?.ID, send]);

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [messages]);

  const sendMessage = () => {
    const content = input.trim();
    if (!content || !conversation) return;
    setInput('');
    send({ type: 'send_message', conversation_id: conversation.ID, content });
  };

  const onKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      sendMessage();
    }
  };

  if (!conversation) {
    return (
      <main className="chat-main empty">
        <div className="empty-state">
          <span className="empty-icon">💬</span>
          <p>Select a conversation to start chatting</p>
        </div>
      </main>
    );
  }

  const otherId = conversation.User1ID === currentUserId ? conversation.User2ID : conversation.User1ID;

  return (
    <main className="chat-main">
      <header className="chat-header">
        {onBack && (
          <button className="back-btn" onClick={onBack} aria-label="Back">
            ‹
          </button>
        )}
        <div className="chat-header-avatar">👤</div>
        <div className="chat-header-info">
          <span className="chat-header-name">{conversation.other_user_name || `User #${otherId}`}</span>
          <span className="chat-header-sub">Match #{conversation.MatchID}</span>
        </div>
      </header>

      <div className="messages-area">
        {loading && <p className="loading">Loading messages…</p>}
        {messages.map((msg) => (
          <MessageBubble key={msg.ID} message={msg} isOwn={msg.SenderID === currentUserId} />
        ))}
        <div ref={bottomRef} />
      </div>

      <footer className="input-area">
        <textarea
          className="message-input"
          placeholder="Type a message…"
          value={input}
          onChange={(e) => setInput(e.target.value)}
          onKeyDown={onKeyDown}
          rows={1}
        />
        <button className="send-btn" onClick={sendMessage} disabled={!input.trim()}>
          ➤
        </button>
      </footer>
    </main>
  );
}
