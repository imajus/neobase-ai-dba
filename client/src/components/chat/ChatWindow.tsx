import { EventSourcePolyfill } from 'event-source-polyfill';
import { ArrowDown, XCircle } from 'lucide-react';
import { useCallback, useEffect, useRef, useState } from 'react';
import toast from 'react-hot-toast';
import { useStream } from '../../contexts/StreamContext';
import axios from '../../services/axiosConfig';
import { Chat, Connection } from '../../types/chat';
import { MessagesResponse, transformBackendMessage } from '../../types/messages';
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
  onSendMessage: (message: string) => Promise<void>;
  onClearChat: () => void;
  onCloseConnection: () => void;
  onEditConnection?: (id: string, connection: Connection) => void;
  onConnectionStatusChange?: (chatId: string, isConnected: boolean, from: string) => void;
  isConnected: boolean;
  eventSource: EventSourcePolyfill | null;
  onCancelStream: () => Promise<void>;
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
  onConnectionStatusChange,
  isConnected,
  eventSource,
  onCancelStream
}: ChatWindowProps) {
  const queryTimeouts = useRef<Record<string, NodeJS.Timeout>>({});
  const [editingMessageId, setEditingMessageId] = useState<string | null>(null);
  const [editInput, setEditInput] = useState('');
  const [showClearConfirm, setShowClearConfirm] = useState(false);
  const [showCloseConfirm, setShowCloseConfirm] = useState(false);
  const [showScrollButton, setShowScrollButton] = useState(false);
  const [queryStates, setQueryStates] = useState<Record<string, QueryState>>({});
  const [isConnecting, setIsConnecting] = useState(false);
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const chatContainerRef = useRef<HTMLDivElement>(null);
  const [showEditConnection, setShowEditConnection] = useState(false);
  const { streamId, generateStreamId } = useStream();
  const [page, setPage] = useState(1);
  const [hasMore, setHasMore] = useState(true);
  const [isLoadingMessages, setIsLoadingMessages] = useState(false);
  const pageSize = 20; // Messages per page
  const loadingRef = useRef<HTMLDivElement>(null);
  const [isMessageSending, setIsMessageSending] = useState(false);

  useEffect(() => {
    if (isConnected) {
      setIsConnecting(false);
    }
  }, [isConnected]);

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
    setShowCloseConfirm(false);
  }, []);

  const handleReconnect = useCallback(async () => {
    try {
      setIsConnecting(true);
      let currentStreamId = streamId;

      // Generate new streamId if not available
      if (!currentStreamId) {
        currentStreamId = generateStreamId();
      }

      // Check if the connection is already established
      const connectionStatus = await checkConnectionStatus(chat.id, currentStreamId);
      if (!connectionStatus) {
        await connectToDatabase(chat.id, currentStreamId);
      }
      console.log('connectionStatus', connectionStatus);
      onConnectionStatusChange?.(chat.id, true, 'chat-window-reconnect');
    } catch (error) {
      console.error('Failed to reconnect to database:', error);
      onConnectionStatusChange?.(chat.id, false, 'chat-window-reconnect');
      toast.error('Failed to reconnect to database', {
        style: {
          background: '#ff4444',
          color: '#fff',
          border: '4px solid #cc0000',
          borderRadius: '12px',
          boxShadow: '4px 4px 0px 0px rgba(0,0,0,1)',
          padding: '12px 24px',
        }
      });
    } finally {
      setIsConnecting(false);
    }
  }, [chat.id, streamId, generateStreamId, onConnectionStatusChange]);

  const checkConnectionStatus = async (chatId: string, streamId: string) => {
    try {
      const response = await axios.get(
        `${import.meta.env.VITE_API_URL}/chats/${chatId}/connection-status`,
        {
          headers: {
            'Content-Type': 'application/json',
            'Authorization': `Bearer ${localStorage.getItem('token')}`
          }
        }
      );
      return response.data;
    } catch (error) {
      console.error('Failed to check connection status:', error);
      return false;
    }
  };

  const handleDisconnect = useCallback(async () => {
    try {
      await axios.post(
        `${import.meta.env.VITE_API_URL}/chats/${chat.id}/disconnect`,
        {
          stream_id: streamId
        },
        {
          headers: {
            'Content-Type': 'application/json',
            'Authorization': `Bearer ${localStorage.getItem('token')}`
          }
        }
      );
      onConnectionStatusChange?.(chat.id, false, 'chat-window-disconnect');
      onCloseConnection();
      handleCloseConfirm();
    } catch (error) {
      console.error('Failed to disconnect:', error);
      toast.error('Failed to disconnect from database');
    }
  }, [chat.id, onCloseConnection, handleCloseConfirm, onConnectionStatusChange]);

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

  const connectToDatabase = async (chatId: string, streamId: string) => {
    try {
      await axios.post(
        `${import.meta.env.VITE_API_URL}/chats/${chatId}/connect`,
        { stream_id: streamId },
        {
          headers: {
            'Content-Type': 'application/json',
            'Authorization': `Bearer ${localStorage.getItem('token')}`
          }
        }
      );
    } catch (error) {
      console.error('Failed to connect to database:', error);
      throw error;
    }
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
        if (aiResponse && aiResponse.type === 'assistant') {
          updated[messageIndex + 1] = aiResponse;
        }
        return updated;
      });
    }
    setEditingMessageId(null);
    setEditInput('');
  }, [messages, setMessages]);

  const fetchMessages = useCallback(async (pageNum: number) => {
    if (!chat.id || isLoadingMessages) return;

    try {
      setIsLoadingMessages(true);
      const response = await axios.get<MessagesResponse>(
        `${import.meta.env.VITE_API_URL}/chats/${chat.id}/messages?page=${pageNum}&page_size=${pageSize}`,
        {
          withCredentials: true,
          headers: {
            'Content-Type': 'application/json',
            'Authorization': `Bearer ${localStorage.getItem('token')}`
          }
        }
      );

      if (response.data.success) {
        const newMessages = response.data.data.messages.map(transformBackendMessage);

        // Prepend messages to existing ones
        setMessages(prev => {
          const combined = [...newMessages, ...prev];
          // Remove duplicates based on message ID
          return Array.from(new Map(combined.map(m => [m.id, m])).values());
        });

        setHasMore(response.data.data.total > pageNum * pageSize);
      }
    } catch (error) {
      console.error('Failed to fetch messages:', error);
      toast.error('Failed to load messages');
    } finally {
      setIsLoadingMessages(false);
    }
  }, [chat.id]);

  // Setup intersection observer for infinite scroll
  useEffect(() => {
    const observer = new IntersectionObserver(
      (entries) => {
        if (entries[0].isIntersecting && hasMore && !isLoadingMessages) {
          setPage(prev => prev + 1);
        }
      },
      { threshold: 0.5 }
    );

    if (loadingRef.current) {
      observer.observe(loadingRef.current);
    }

    return () => observer.disconnect();
  }, [hasMore, isLoadingMessages]);

  // Fetch messages when page changes
  useEffect(() => {
    fetchMessages(page);
  }, [page, fetchMessages]);

  // Reset pagination when chat changes
  useEffect(() => {
    setPage(1);
    setHasMore(true);
    setMessages([]);
  }, [chat.id, setMessages]);

  // Remove handleSendMessage and use the prop instead
  const handleMessageSubmit = async (content: string) => {
    if (isMessageSending) return;
    setIsMessageSending(true);
    try {
      await onSendMessage(content);
    } finally {
      setIsMessageSending(false);
    }
  };

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
        <div ref={loadingRef} className="w-full py-4 text-center">
          {isLoadingMessages && (
            <div className="flex items-center justify-center gap-2">
              <div className="w-4 h-4 border-2 border-black border-t-transparent rounded-full animate-spin" />
              <span className="text-sm text-gray-600">Loading messages...</span>
            </div>
          )}
        </div>

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
          {[...messages].reverse().map((message) => (
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

        {messages.some(m => m.isStreaming) && (
          <div className="
            fixed 
            bottom-[88px]  // Position it above message input
            left-1/2 
            -translate-x-1/2 
            z-50
          ">
            <button
              onClick={onCancelStream}
              className="
                neo-border
                px-3
                py-1.5
                flex
                items-center
                gap-1.5
                bg-white
                text-sm
                font-medium
                hover:bg-red-50
                active:translate-y-[1px]
                active:shadow-[2px_2px_0px_0px_rgba(0,0,0,1)]
              "
            >
              <XCircle className="w-3.5 h-3.5" />
              <span>Cancel</span>
            </button>
          </div>
        )}

        {showScrollButton && (
          <button
            onClick={scrollToBottom}
            className="fixed bottom-24 right-8 p-3 bg-black text-white rounded-full shadow-lg hover:bg-gray-800 transition-all neo-border z-40"
            title="Scroll to bottom"
          >
            <ArrowDown className="w-6 h-6" />
          </button>
        )}
      </div>

      <MessageInput
        isConnected={isConnected}
        onSendMessage={handleMessageSubmit}
        isExpanded={isExpanded}
        isDisabled={isMessageSending}
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
            onEdit={async (data) => {
              await onEditConnection?.(chat.id, data);
              setShowEditConnection(false);
              return { success: true };
            }}
            onSubmit={async (data) => {
              await onEditConnection?.(chat.id, data);
              setShowEditConnection(false);
            }}
          />
        </div>
      )}
    </div>
  );
}