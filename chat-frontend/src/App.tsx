import { useState, useEffect, useMemo } from 'react';
import type { Conversation } from './types/chat';
import { chatApi } from './api/chat';
import { ConversationList } from './components/ConversationList';
import { ChatWindow } from './components/ChatWindow';
import './index.css';

function parseAuthFromURL(): { userId: number; token: string; matchId: number | null } | null {
  const params = new URLSearchParams(window.location.search);
  const userIdStr = params.get('user_id');
  const token = params.get('token');
  if (!userIdStr || !token) return null;
  const userId = parseInt(userIdStr, 10);
  if (isNaN(userId)) return null;
  const matchIdStr = params.get('match_id');
  const matchId = matchIdStr ? parseInt(matchIdStr, 10) : null;
  return { userId, token, matchId: isNaN(matchId ?? NaN) ? null : matchId };
}

export default function App() {
  const auth = useMemo(parseAuthFromURL, []);

  const [conversations, setConversations] = useState<Conversation[]>([]);
  const [selected, setSelected] = useState<Conversation | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [showChat, setShowChat] = useState(false);

  useEffect(() => {
    if (!auth) return;

    chatApi
      .getConversations(auth.userId)
      .then((convs) => {
        setConversations(convs ?? []);

        if (auth.matchId && convs?.length) {
          const target = convs.find((c) => c.MatchID === auth.matchId);
          if (target) { setSelected(target); setShowChat(true); }
        } else if (convs?.length) {
          setSelected(convs[0]);
          setShowChat(true);
        }
      })
      .catch((err: Error) => setError(err.message));
  }, [auth?.userId, auth?.matchId]);

  const handleSelect = (conv: Conversation) => {
    setSelected(conv);
    setShowChat(true);
  };

  const handleBack = () => setShowChat(false);

  if (!auth) {
    return (
      <div className="auth-error">
        <h2>🔒 Authentication required</h2>
        <p>Open this page through the AltDating bot after getting a match.</p>
      </div>
    );
  }

  if (error) {
    return (
      <div className="auth-error">
        <h2>⚠️ Error</h2>
        <p>{error}</p>
      </div>
    );
  }

  return (
    <div className={`app ${showChat ? 'app--chat-open' : ''}`}>
      <ConversationList
        conversations={conversations}
        selectedId={selected?.ID ?? null}
        currentUserId={auth.userId}
        onSelect={handleSelect}
      />
      <ChatWindow
        conversation={selected}
        currentUserId={auth.userId}
        token={auth.token}
        onBack={handleBack}
      />
    </div>
  );
}
