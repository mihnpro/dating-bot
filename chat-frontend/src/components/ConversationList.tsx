import type { Conversation } from '../types/chat';

interface Props {
  conversations: Conversation[];
  selectedId: string | null;
  currentUserId: number;
  onSelect: (conv: Conversation) => void;
}

export function ConversationList({ conversations, selectedId, currentUserId, onSelect }: Props) {
  return (
    <aside className="sidebar">
      <div className="sidebar-header">
        <h2>💞 Chats</h2>
      </div>
      {conversations.length === 0 && (
        <p className="empty-state">No conversations yet</p>
      )}
      <ul className="conv-list">
        {conversations.map((conv) => {
          const otherId = conv.User1ID === currentUserId ? conv.User2ID : conv.User1ID;
          return (
            <li
              key={conv.ID}
              className={`conv-item ${selectedId === conv.ID ? 'active' : ''}`}
              onClick={() => onSelect(conv)}
            >
              <div className="conv-avatar">👤</div>
              <div className="conv-info">
                <span className="conv-name">{conv.other_user_name || `User #${otherId}`}</span>
                <span className="conv-sub">Match #{conv.MatchID}</span>
              </div>
            </li>
          );
        })}
      </ul>
    </aside>
  );
}
