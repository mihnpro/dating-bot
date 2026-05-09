import type { Message } from '../types/chat';

interface Props {
  message: Message;
  isOwn: boolean;
}

function formatTime(iso: string): string {
  return new Date(iso).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
}

export function MessageBubble({ message, isOwn }: Props) {
  return (
    <div className={`bubble-wrap ${isOwn ? 'own' : 'other'}`}>
      <div className="bubble">
        <p>{message.Content}</p>
        <span className="bubble-time">{formatTime(message.SentAt)}</span>
      </div>
    </div>
  );
}
