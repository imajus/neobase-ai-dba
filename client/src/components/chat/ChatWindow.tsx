import { ArrowDown, Loader2, XCircle } from 'lucide-react';
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
  onEditMessage

  : (id: string, content: string) => void;
  onClearChat: () => void;
  onCloseConnection: () => void;
  onEditConnection?: (id: string, connection: Connection) => Promise<{ success: boolean, error?: string }>;
  onConnectionStatusChange?: (chatId: string, isConnected: boolean, from: string) => void;
  isConnected: boolean;
  onCancelStream: () => Promise<void>;
  onRefreshSchema: () => Promise<void>;
  checkSSEConnection: () => Promise<void>;
}

interface QueryState {
  isExecuting: boolean;
  isExample: boolean;
}

const formatMessageTime = (dateString: string) => {
  const date = new Date(dateString);
  return date.toLocaleTimeString('en-US', {
    hour: 'numeric',
    minute: 'numeric',
    hour12: true
  });
};

const formatDateDivider = (dateString: string) => {
  const date = new Date(dateString);
  const today = new Date();
  const yesterday = new Date(today);
  yesterday.setDate(yesterday.getDate() - 1);

  if (date.toDateString() === today.toDateString()) {
    return 'Today';
  } else if (date.toDateString() === yesterday.toDateString()) {
    return 'Yesterday';
  }
  return date.toLocaleDateString('en-US', {
    month: 'long',
    day: 'numeric',
    year: 'numeric'
  });
};

const groupMessagesByDate = (messages: Message[]) => {
  const groups: { [key: string]: Message[] } = {};

  // Sort messages by date, oldest first
  const sortedMessages = [...messages].sort((a, b) =>
    new Date(a.created_at).getTime() - new Date(b.created_at).getTime()
  );

  sortedMessages.forEach(message => {
    const date = new Date(message.created_at).toDateString();
    if (!groups[date]) {
      groups[date] = [];
    }
    groups[date].push(message);
  });

  // Convert to array and sort by date
  const sortedEntries = Object.entries(groups).sort((a, b) =>
    new Date(a[0]).getTime() - new Date(b[0]).getTime()
  );

  return Object.fromEntries(sortedEntries);
};

export default function ChatWindow({
  chat,
  onEditMessage,
  isExpanded,
  messages,
  setMessages,
  onSendMessage,
  onClearChat,
  onCloseConnection,
  onEditConnection,
  onConnectionStatusChange,
  isConnected,
  onCancelStream,
  onRefreshSchema,
  checkSSEConnection
}: ChatWindowProps) {
  const queryTimeouts = useRef<Record<string, NodeJS.Timeout>>({});
  const [editingMessageId, setEditingMessageId] = useState<string | null>(null);
  const [editInput, setEditInput] = useState('');
  const [showClearConfirm, setShowClearConfirm] = useState(false);
  const [showRefreshSchema, setShowRefreshSchema] = useState(false);
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
  const pageSize = 40; // Messages per page
  const loadingRef = useRef<HTMLDivElement>(null);
  const [isMessageSending, setIsMessageSending] = useState(false);
  const prevMessageCountRef = useRef(messages.length);
  const isLoadingOldMessages = useRef(false);
  const messageUpdateSource = useRef<'api' | 'new' | null>(null);
  const isInitialLoad = useRef(true);

  useEffect(() => {
    if (isConnected) {
      setIsConnecting(false);
    }
  }, [isConnected]);

  const scrollToBottom = (origin: string) => {
    console.log("scrollToBottom called from", origin);
    requestAnimationFrame(() => {
      if (chatContainerRef.current) {
        const scrollOptions = {
          top: chatContainerRef.current.scrollHeight,
          behavior: origin === 'initial' ? 'auto' : 'smooth'
        } as ScrollToOptions;

        try {
          chatContainerRef.current.scrollTo(scrollOptions);
        } catch (error) {
          // Fallback for older browsers
          chatContainerRef.current.scrollTop = chatContainerRef.current.scrollHeight;
        }
      }
    });
  };

  useEffect(() => {
    const chatContainer = chatContainerRef.current;
    if (!chatContainer) return;

    const observer = new MutationObserver(() => {
      const { scrollTop, scrollHeight, clientHeight } = chatContainer;
      const isNearBottom = scrollHeight - scrollTop - clientHeight < 200;
      setShowScrollButton(!isNearBottom);

      if ((isNearBottom || messages.some(m => m.is_streaming))
        && !isLoadingOldMessages.current
        && !isLoadingMessages
        && messageUpdateSource.current !== 'api') {
        scrollToBottom('mutation-observer');
      }
    });

    observer.observe(chatContainer, {
      childList: true,
      subtree: true,
      characterData: true
    });

    return () => observer.disconnect();
  }, [messages, isLoadingMessages]);

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
      onEditMessage(id, content);
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

  const fetchMessages = useCallback(async (page: number) => {
    if (!chat?.id || isLoadingMessages) return;

    try {
      console.log('Fetching messages, page:', page);
      setIsLoadingMessages(true);
      isLoadingOldMessages.current = page > 1;
      messageUpdateSource.current = 'api';

      const response = await axios.get<MessagesResponse>(
        `${import.meta.env.VITE_API_URL}/chats/${chat.id}/messages?page=${page}&page_size=${pageSize}`,
        {
          withCredentials: true,
          headers: {
            'Authorization': `Bearer ${localStorage.getItem('token')}`
          }
        }
      );

      if (response.data.success) {
        const newMessages = response.data.data.messages.map(transformBackendMessage);
        console.log('Received messages:', newMessages.length, 'for page:', page);

        if (page === 1) {
          // For initial load, just set messages and scroll to bottom
          setMessages(newMessages);
          requestAnimationFrame(() => {
            scrollToBottom('initial-load');
          });
        } else {
          // For pagination, preserve scroll position
          const container = chatContainerRef.current;
          if (container) {
            const oldHeight = container.scrollHeight;
            const oldScroll = container.scrollTop;

            setMessages(prev => [...newMessages, ...prev]);

            // After React has updated the DOM
            requestAnimationFrame(() => {
              // Calculate how much new content was added
              const newHeight = container.scrollHeight;
              const heightDiff = newHeight - oldHeight;
              // Adjust scroll position to show same content as before
              container.scrollTop = oldScroll + heightDiff;
            });
          }
        }

        setHasMore(newMessages.length === pageSize);
        if (page === 1) isInitialLoad.current = false;
      }
    } catch (error) {
      console.error('Failed to fetch messages:', error);
      toast.error('Failed to load messages');
    } finally {
      setTimeout(() => {
        messageUpdateSource.current = null;
        isLoadingOldMessages.current = false;
        setIsLoadingMessages(false);
      }, 100);
    }
  }, [chat?.id, pageSize]);

  // Update intersection observer effect
  useEffect(() => {
    const observer = new IntersectionObserver(
      (entries) => {
        // Only fetch more if:
        // 1. Loading element is visible
        // 2. We have more messages to load
        // 3. We're not currently loading
        if (entries[0].isIntersecting &&
          hasMore &&
          !isLoadingMessages) {
          console.log('Loading more messages, current page:', page);
          setPage(prev => prev + 1);
          fetchMessages(page + 1);  // Fetch next page immediately
        }
      },
      {
        root: null,
        rootMargin: '100px',  // Start loading before element is visible
        threshold: 0.1
      }
    );

    if (loadingRef.current) {
      observer.observe(loadingRef.current);
    }

    return () => observer.disconnect();
  }, [hasMore, isLoadingMessages, page, fetchMessages]);

  // Keep only necessary effects
  useEffect(() => {
    if (chat?.id) {
      console.log('Chat changed, loading initial messages');
      isInitialLoad.current = true;
      setPage(1);
      setHasMore(true);
      setMessages([]);
      fetchMessages(1);
    }
  }, [chat?.id, fetchMessages]);

  // Update the message update effect
  useEffect(() => {
    // Skip effect if source is API or loading old messages
    if (messageUpdateSource.current === 'api' || isLoadingOldMessages.current) {
      console.log('Skipping scroll - API/pagination update');
      return;
    }

    // Only scroll for new messages or streaming
    const lastMessage = messages[messages.length - 1];
    if (lastMessage?.is_streaming || lastMessage?.type === 'user') {
      console.log('Scrolling - new message/streaming');
      scrollToBottom('new-message');
    }
  }, [messages]);

  // Update the handleMessageSubmit function
  const handleMessageSubmit = async (content: string) => {
    try {
      messageUpdateSource.current = 'new';
      await onSendMessage(content);
      scrollToBottom('message-submit');
    } finally {
      messageUpdateSource.current = null;
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
        setShowRefreshSchema={() => setShowRefreshSchema(true)}
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
          ref={loadingRef}
          className="h-20 flex items-center justify-center"
        >
          {isLoadingMessages && (
            <div className="flex items-center justify-center gap-2">
              <Loader2 className="w-4 h-4 animate-spin" />
              <span className="text-sm text-gray-600">Loading older messages...</span>
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
          {Object.entries(groupMessagesByDate(messages)).map(([date, dateMessages], index) => (
            <div key={date}>
              <div className={`flex items-center justify-center ${index === 0 ? 'mb-4' : 'my-6'}`}>
                <div className="
                  px-4 
                  py-2
                  bg-white 
                  text-sm 
                  font-medium 
                  text-black
                  border-2
                  border-black
                  shadow-[2px_2px_0px_0px_rgba(0,0,0,1)]
                  rounded-full
                ">
                  {formatDateDivider(date)}
                </div>
              </div>

              {dateMessages.map((message, index) => (
                <MessageTile
                  key={message.id}
                  checkSSEConnection={checkSSEConnection}
                  chatId={chat.id}
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
                  isFirstMessage={index === 0}
                />
              ))}
            </div>
          ))}
        </div>
        <div ref={messagesEndRef} />

        {messages.some(m => m.is_streaming) && (
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
                py-2
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
              <span>Cancel Request</span>
            </button>
          </div>
        )}

        {showScrollButton && (
          <button
            onClick={() => scrollToBottom('scroll-button')}
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

      {showRefreshSchema && (
        <ConfirmationModal
          title="Refresh Knowledge Base"
          message="Are you sure you want to refresh the knowledge base? This action will refetch the schema from the database and update the knowledge base."
          onConfirm={async () => {
            await onRefreshSchema();
            setShowRefreshSchema(false);
          }}
          onCancel={() => setShowRefreshSchema(false)}
        />
      )}

      {showClearConfirm && (
        <ConfirmationModal
          title="Clear Chat"
          message="Are you sure you want to clear all chat messages? This action cannot be undone."
          onConfirm={async () => {
            await onClearChat();
            setShowClearConfirm(false);
          }}
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
              const result = await onEditConnection?.(chat.id, data);
              return { success: result?.success || false, error: result?.error };
            }}
            onSubmit={async (data) => {
              const result = await onEditConnection?.(chat.id, data);
              return { success: result?.success || false, error: result?.error };
            }}
          />
        </div>
      )}
    </div>
  );
}