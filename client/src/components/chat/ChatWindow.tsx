import { ArrowDown } from 'lucide-react';
import { useCallback, useEffect, useRef, useState } from 'react';
import toast from 'react-hot-toast';
import { Chat } from '../../types/chat';
import ConfirmationModal from '../modals/ConfirmationModal';
import ConnectionModal from '../modals/ConnectionModal';
import ChatHeader from './ChatHeader';
import MessageInput from './MessageInput';
import MessageTile from './MessageTile';
import { Message } from './types';
interface ChatWindowProps {
  chat: Chat;
  isExpanded: boolean;
  messages: Message[];
  setMessages: React.Dispatch<React.SetStateAction<Message[]>>;
  onSendMessage: (message: string) => void;
  onClearChat: () => void;
  onCloseConnection: () => void;
  onEditConnection?: (id: string, data: Chat) => void;
}

interface QueryState {
  isExecuting: boolean;
  isExample: boolean;
}

export default function ChatWindow({
  chat,
  isExpanded,
  messages,
  setMessages,
  onSendMessage,
  onClearChat,
  onCloseConnection,
  onEditConnection,
}: ChatWindowProps) {
  const queryTimeouts = useRef<Record<string, NodeJS.Timeout>>({});
  const [editingMessageId, setEditingMessageId] = useState<string | null>(null);
  const [editInput, setEditInput] = useState('');
  const [showClearConfirm, setShowClearConfirm] = useState(false);
  const [showCloseConfirm, setShowCloseConfirm] = useState(false);
  const [showScrollButton, setShowScrollButton] = useState(false);
  const [queryStates, setQueryStates] = useState<Record<string, QueryState>>({});
  const [isConnected, setIsConnected] = useState(true);
  const [isConnecting, setIsConnecting] = useState(true);
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const chatContainerRef = useRef<HTMLDivElement>(null);
  const [showEditConnection, setShowEditConnection] = useState(false);

  useEffect(() => {
    // Simulate connection establishment
    const timer = setTimeout(() => {
      setIsConnecting(false);
    }, 2000);
    return () => clearTimeout(timer);
  }, []);

  const scrollToBottom = () => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  };

  useEffect(() => {
    const chatContainer = chatContainerRef.current;
    if (!chatContainer) return;

    const observer = new MutationObserver(() => {
      // Only auto-scroll if user is already at bottom
      const { scrollTop, scrollHeight, clientHeight } = chatContainer;
      const isNearBottom = scrollHeight - scrollTop - clientHeight < 100;

      if (isNearBottom) {
        scrollToBottom();
      }
    });

    observer.observe(chatContainer, {
      childList: true,
      subtree: true,
      characterData: true
    });

    return () => observer.disconnect();
  }, []);

  const handleScroll = useCallback(() => {
    if (!chatContainerRef.current) return;

    const { scrollTop, scrollHeight, clientHeight } = chatContainerRef.current;
    const isNearBottom = scrollHeight - scrollTop - clientHeight < 100;
    setShowScrollButton(!isNearBottom);
  }, []);

  useEffect(() => {
    scrollToBottom();
  }, [messages]);

  useEffect(() => {
    const chatContainer = chatContainerRef.current;
    if (chatContainer) {
      chatContainer.addEventListener('scroll', handleScroll);
      return () => chatContainer.removeEventListener('scroll', handleScroll);
    }
  }, [handleScroll]);

  const handleClearConfirm = useCallback(() => {
    onClearChat();
    setShowClearConfirm(false);
  }, [onClearChat]);

  const handleCloseConfirm = useCallback(() => {
    setIsConnected(false);
    setShowCloseConfirm(false);
  }, []);

  const handleReconnect = useCallback(() => {
    setIsConnected(true);
    toast.success('Reconnected to database', {
      style: {
        background: '#000',
        color: '#fff',
        border: '4px solid #000',
        borderRadius: '12px',
        boxShadow: '4px 4px 0px 0px rgba(0,0,0,1)',
        padding: '12px 24px',
        fontSize: '14px',
        fontWeight: '500',
      },
      position: 'bottom-center',
      duration: 2000,
    });
  }, []);

  const handleDisconnect = useCallback(() => {
    onCloseConnection();
    handleCloseConfirm();
  }, [onCloseConnection, handleCloseConfirm]);

  const handleEditMessage = (id: string) => {
    // Prevent auto-scroll
    const message = messages.find(m => m.id === id);
    if (message) {
      setEditingMessageId(id);
      setEditInput(message.content);
    }
  };

  const handleCancelEdit = () => {
    // Prevent auto-scroll
    setEditingMessageId(null);
    setEditInput('');
  };

  const handleSaveEdit = useCallback((id: string, content: string) => {
    if (content.trim()) {
      // Find the message and its index
      const messageIndex = messages.findIndex(msg => msg.id === id);
      if (messageIndex === -1) return;

      // Get the edited message and the next message (AI response)
      const editedMessage = messages[messageIndex];
      const aiResponse = messages[messageIndex + 1];

      setMessages(prev => {
        const updated = [...prev];
        // Update the edited message
        updated[messageIndex] = { ...editedMessage, content: content.trim() };
        // Keep the AI response if it exists
        if (aiResponse && aiResponse.type === 'ai') {
          updated[messageIndex + 1] = aiResponse;
        }
        return updated;
      });
    }
    setEditingMessageId(null);
    setEditInput('');
  }, [messages, setMessages]);

  return (
    <div className={`flex-1 flex flex-col h-screen transition-all duration-300 relative ${isExpanded ? 'md:ml-80' : 'md:ml-20'}`}>
      <ChatHeader
        chat={chat}
        isConnecting={isConnecting}
        isConnected={isConnected}
        onClearChat={() => setShowClearConfirm(true)}
        onEditConnection={() => setShowEditConnection(true)}
        onShowCloseConfirm={() => setShowCloseConfirm(true)}
        onReconnect={handleReconnect}
      />

      <div
        ref={chatContainerRef}
        className="
          flex-1 
          overflow-y-auto 
          bg-[#FFDB58]/10 
          relative 
          scroll-smooth 
          pb-24 
          md:pb-32 
          mt-16
          md:mt-0
        "
      >
        <div
          className={`
            max-w-5xl 
            mx-auto
            px-4
            pt-16
            md:pt-0
            md:px-2
            xl:px-0
            transition-all 
            duration-300
            ${isExpanded
              ? 'md:ml-6 lg:ml-6 xl:mx-8 [@media(min-width:1760px)]:ml-[4rem] [@media(min-width:1920px)]:ml-[8.4rem]'
              : 'md:ml-[19rem] xl:mx-auto'
            }
          `}
        >
          {messages.map((message) => (
            <MessageTile
              key={message.id}
              message={message}
              onEdit={handleEditMessage}
              editingMessageId={editingMessageId}
              editInput={editInput}
              setEditInput={setEditInput}
              onSaveEdit={handleSaveEdit}
              onCancelEdit={handleCancelEdit}
              queryStates={queryStates}
              setQueryStates={setQueryStates}
              queryTimeouts={queryTimeouts}
            />
          ))}
        </div>
        <div ref={messagesEndRef} />

        {showScrollButton && (
          <button
            onClick={scrollToBottom}
            className="fixed bottom-24 z-[10] right-8 p-3 bg-black text-white rounded-full shadow-lg hover:bg-gray-800 transition-all neo-border"
            title="Scroll to bottom"
          >
            <ArrowDown className="w-6 h-6" />
          </button>
        )}
      </div>

      <MessageInput
        isConnected={isConnected}
        onSendMessage={onSendMessage}
        isExpanded={isExpanded}
      />

      {showClearConfirm && (
        <ConfirmationModal
          title="Clear Chat"
          message="Are you sure you want to clear all chat messages? This action cannot be undone."
          onConfirm={handleClearConfirm}
          onCancel={() => setShowClearConfirm(false)}
        />
      )}

      {showCloseConfirm && (
        <ConfirmationModal
          title="Disconnect Connection"
          message="Are you sure you want to disconnect from this database?"
          onConfirm={handleDisconnect}
          onCancel={() => setShowCloseConfirm(false)}
        />
      )}

      {showEditConnection && (
        <div className="fixed inset-0 z-[100] flex items-center justify-center bg-black/50">
          <ConnectionModal
            initialData={chat}
            onClose={() => setShowEditConnection(false)}
            onEdit={(data) => {
              onEditConnection?.(data.id, data);
              setShowEditConnection(false);
            }}
            onSubmit={(data) => {
              onEditConnection?.(chat.id, data);
              setShowEditConnection(false);
            }}
          />
        </div>
      )}
    </div>
  );
}