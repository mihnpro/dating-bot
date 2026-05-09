export interface Conversation {
  ID: string;
  MatchID: number;
  User1ID: number;
  User2ID: number;
  Status: 'active' | 'blocked';
  CreatedAt: string;
  UpdatedAt: string;
  other_user_name?: string;
  other_username?: string;
}

export interface Message {
  ID: string;
  ConversationID: string;
  SenderID: number;
  Content: string;
  ContentType: 'text';
  SentAt: string;
  IsRead: boolean;
}

// Shape of a message as delivered over WebSocket (snake_case from Go)
export interface WSMessage {
  id: string;
  conversation_id: string;
  sender_id: number;
  content: string;
  sent_at: string;
}

// WebSocket messages sent by the server
export type WSServerEvent =
  | { type: 'new_message'; message: WSMessage }
  | { type: 'message_read'; conversation_id: string; reader_id: number }
  | { type: 'pong' }
  | { type: 'error'; error: string };

// WebSocket messages sent by the client
export type WSClientMessage =
  | { type: 'send_message'; conversation_id: string; content: string }
  | { type: 'mark_read'; conversation_id: string }
  | { type: 'ping' };

export interface AuthParams {
  token: string;
  userId: number;
  matchId: number | null;
}
