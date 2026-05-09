import type { Conversation, Message } from '../types/chat';

const BASE = '/api/v1/chat';

async function get<T>(url: string): Promise<T> {
  const res = await fetch(url);
  if (!res.ok) throw new Error(`HTTP ${res.status}: ${await res.text()}`);
  return res.json() as Promise<T>;
}

async function post<T>(url: string, body: unknown): Promise<T> {
  const res = await fetch(url, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  });
  if (!res.ok) throw new Error(`HTTP ${res.status}: ${await res.text()}`);
  return res.json() as Promise<T>;
}

export const chatApi = {
  getConversations(userId: number): Promise<Conversation[]> {
    return get<Conversation[]>(`${BASE}/conversations?user_id=${userId}`);
  },

  getOrCreateConversation(matchId: number, user1Id: number, user2Id: number): Promise<Conversation> {
    return post<Conversation>(`${BASE}/conversations`, {
      match_id: matchId,
      user1_id: user1Id,
      user2_id: user2Id,
    });
  },

  getMessages(convId: string, limit = 50, offset = 0): Promise<Message[]> {
    return get<Message[]>(`${BASE}/conversations/${convId}/messages?limit=${limit}&offset=${offset}`);
  },
};
