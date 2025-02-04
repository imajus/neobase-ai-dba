import { AlertCircle, ArrowDown, Braces, Clock, Copy, Eraser, History, Loader, Pencil, Play, PlugZap, RefreshCw, Send, Table, X, XCircle } from 'lucide-react';
import React, { useCallback, useEffect, useRef, useState } from 'react';
import toast from 'react-hot-toast';
import DatabaseLogo from '../icons/DatabaseLogos';
import ConfirmationModal from '../modals/ConfirmationModal';

const toastStyle = {
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
  position: 'bottom-center' as const,
  duration: 2000,
};

export interface Message {
  id: string;
  content: string;
  type: 'user' | 'ai';
  isLoading?: boolean;
  sql?: string;
  executionTime?: number;
  result?: any[] | null;
  error?: {
    code: string;
    message: string;
    details?: string;
  };
}

interface ChatWindowProps {
  connectionName: string;
  connectionType: 'postgresql' | 'mysql' | 'clickhouse' | 'mongodb' | 'redis' | 'neo4j';
  isExpanded: boolean;
  messages: Message[];
  onSendMessage: (message: string) => void;
  setMessages?: (messages: Message[]) => void;
  onClearChat: () => void;
  onCloseConnection: () => void;
}

interface QueryState {
  isExecuting: boolean;
  isExample: boolean;
}

export default function ChatWindow({
  connectionName,
  connectionType,
  isExpanded,
  messages,
  setMessages,
  onSendMessage,
  onClearChat,
  onCloseConnection,
}: ChatWindowProps) {
  const queryTimeouts = useRef<Record<string, NodeJS.Timeout>>({});
  const [input, setInput] = useState('');
  const [editingMessageId, setEditingMessageId] = useState<string | null>(null);
  const [editInput, setEditInput] = useState('');
  const [viewMode, setViewMode] = useState<'table' | 'json'>('table');
  const [showClearConfirm, setShowClearConfirm] = useState(false);
  const [showCloseConfirm, setShowCloseConfirm] = useState(false);
  const [showScrollButton, setShowScrollButton] = useState(false);
  const [queryStates, setQueryStates] = useState<Record<string, QueryState>>({});
  const [isConnected, setIsConnected] = useState(true);
  const [isConnecting, setIsConnecting] = useState(true);
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const chatContainerRef = useRef<HTMLDivElement>(null);

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

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (input.trim()) {
      onSendMessage(input.trim());
      setInput('');
    }
  };

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
    toast.success('Reconnected to database', toastStyle);
  }, []);

  const handleDisconnect = useCallback(() => {
    onCloseConnection();
    setShowCloseConfirm(false);
  }, [onCloseConnection]);

  const handleCopyToClipboard = (text: string) => {
    navigator.clipboard.writeText(text);
    toast.success('Copied to clipboard!', toastStyle);
  };

  const renderTableView = (data: any[]) => {
    if (!data.length) return null;
    const columns = Object.keys(data[0]);

    return (
      <div className="overflow-x-auto">
        <table className="w-full text-left border-collapse">
          <thead>
            <tr>
              {columns.map(column => (
                <th key={column} className="py-2 px-4 bg-gray-800 border-b border-gray-700 text-gray-300 font-mono">
                  {column}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {data.map((row, i) => (
              <tr key={i} className="border-b border-gray-700">
                {columns.map(column => (
                  <td key={column} className="py-2 px-4">
                    <span className={`${typeof row[column] === 'number'
                      ? 'text-cyan-400'
                      : typeof row[column] === 'boolean'
                        ? 'text-purple-400'
                        : column.includes('time') || column.includes('date')
                          ? 'text-yellow-300'
                          : 'text-green-400'
                      }`}>
                      {JSON.stringify(row[column])}
                    </span>
                  </td>
                ))}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    );
  };

  return (
    <div className={`flex-1 flex flex-col h-screen transition-all duration-300 relative ${isExpanded ? 'md:ml-80' : 'md:ml-20'
      }`}>
      <div className="fixed top-0 left-0 right-0 md:relative md:left-auto md:right-auto bg-white border-b-4 border-black h-16 px-4 flex justify-between items-center mt-16 md:mt-0 z-20">
        <div className="flex items-center gap-2 overflow-hidden max-w-[60%]">
          <DatabaseLogo type={connectionType} size={32} className="transition-transform hover:scale-110" />
          <h2 className="text-lg md:text-2xl font-bold truncate">{connectionName}</h2>
          {isConnecting ? (
            <span className="text-yellow-600 text-sm font-medium bg-yellow-100 px-2 py-1 rounded flex items-center gap-2">
              <Loader className="w-3 h-3 animate-spin" />
              Connecting...
            </span>
          ) : !isConnected ? (
            <span className="text-neo-error text-sm font-medium bg-neo-error/10 px-2 py-1 rounded">
              Disconnected
            </span>
          ) : (
            <span className="text-emerald-700 text-sm font-medium bg-emerald-100 px-2 py-1 rounded">
              Connected
            </span>
          )}
        </div>
        <div className="flex items-center gap-2">
          <button
            onClick={() => setShowClearConfirm(true)}
            className="p-2 text-neo-error hover:bg-neo-error/10 rounded-lg transition-colors hidden md:block neo-border"
            title="Clear chat"
          >
            <Eraser className="w-5 h-5" />
          </button>
          {isConnected ? (
            <button
              onClick={() => setShowCloseConfirm(true)}
              className="p-2 hover:bg-neo-gray rounded-lg transition-colors hidden md:block neo-border text-gray-800"
              title="Disconnect connection"
            >
              <PlugZap className="w-5 h-5" />
            </button>
          ) : (
            <button
              onClick={handleReconnect}
              className="p-2 hover:bg-neo-gray rounded-lg transition-colors hidden md:block neo-border"
              title="Reconnect"
            >
              <RefreshCw className="w-5 h-5 text-gray-800" />
            </button>
          )}
          {/* Mobile buttons without borders */}
          <button
            onClick={() => setShowClearConfirm(true)}
            className="p-2 text-neo-error hover:bg-neo-error/10 rounded-lg transition-colors md:hidden"
            title="Clear chat"
          >
            <Eraser className="w-5 h-5" />
          </button>
          {isConnected ? (
            <button
              onClick={() => setShowCloseConfirm(true)}
              className="p-2 hover:bg-neo-gray rounded-lg transition-colors md:hidden"
              title="Disconnect connection"
            >
              <PlugZap className="w-5 h-5 text-gray-800" />
            </button>
          ) : (
            <button
              onClick={handleReconnect}
              className="p-2 hover:bg-neo-gray rounded-lg transition-colors md:hidden"
              title="Reconnect"
            >
              <RefreshCw className="w-5 h-5 text-gray-800" />
            </button>
          )}
        </div>
      </div>

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
            md:px-2
            xl:px-0
            transition-all 
            duration-300
            ${isExpanded
              ? 'md:ml-6 lg:ml-6 xl:mx-8 [@media(min-width:1920px)]:ml-[8rem]'
              : 'md:ml-[19rem] xl:mx-auto'
            }
          `}
        >
          {messages.map((message) => (
            <div key={message.id} className="py-4 md:py-6 first:pt-8">
              <div className={`
                group flex items-center relative
                ${message.type === 'user' ? 'justify-end' : 'justify-start'}
              `}>
                {message.type === 'user' && (
                  <div className="
                    absolute 
                    right-0 
                    -bottom-9
                    md:-bottom-10 
                    flex 
                    gap-1
                    z-[5]
                  ">
                    <button
                      onClick={() => handleCopyToClipboard(message.content)}
                      className="
                        -translate-y-1/2
                        p-1.5
                        md:p-2 
                        group-hover:opacity-100 
                        transition-opacity 
                        hover:bg-black/10 
                        rounded-lg
                        flex-shrink-0
                        border-0
                        bg-white/80
                        backdrop-blur-sm
                      "
                      title="Copy message"
                    >
                      <Copy className="w-4 h-4 text-gray-800" />
                    </button>
                    <button
                      onClick={() => {
                        setEditingMessageId(message.id);
                        setEditInput(message.content);
                      }}
                      className="
                        -translate-y-1/2
                        p-1.5
                        md:p-2
                        group-hover:opacity-100 
                        transition-opacity 
                        hover:bg-black/10
                        rounded-lg
                        flex-shrink-0
                        border-0
                        bg-white/80
                        backdrop-blur-sm
                      "
                      title="Edit message"
                    >
                      <Pencil className="w-4 h-4 text-gray-800" />
                    </button>
                  </div>
                )}
                <div
                  className={`
                    message-bubble
                    inline-block
                    w-[95%]
                    sm:w-[85%]
                    ${message.type === 'user' && editingMessageId === message.id ? "md:w-[75%]" : "md:w-auto"}
                    md:max-w-[75%]
                    ${message.type === 'user'
                      ? 'message-bubble-user'
                      : 'message-bubble-ai'
                    }
                  `}
                >
                  {message.isLoading ? (
                    <div className="flex items-center gap-3">
                      <Loader className="w-5 h-5 animate-spin" />
                      <span className="text-gray-600">Thinking...</span>
                    </div>
                  ) : message.type === 'user' && editingMessageId === message.id ? (
                    <div className='w-full'>
                      <textarea
                        value={editInput}
                        onChange={(e) => setEditInput(e.target.value)}
                        className="
                          neo-input 
                          w-full
                          text-lg
                          min-h-[42px]
                          resize-y
                          py-2
                          px-3
                          leading-normal
                          whitespace-pre-wrap
                        "
                        rows={Math.min(
                          Math.max(
                            editInput.split('\n').length,
                            Math.ceil(editInput.length / 50)
                          ),
                          10
                        )}
                        autoFocus
                      />
                      <div className="flex gap-2 mt-3">
                        <button
                          onClick={() => setEditingMessageId(null)}
                          className="neo-button-secondary flex-1 flex items-center justify-center gap-2"
                        >
                          <X className="w-4 h-4" />
                          <span>Cancel</span>
                        </button>
                        <button
                          onClick={() => {
                            if (editInput.trim()) {
                              const updatedMessages = messages.map(msg =>
                                msg.id === message.id
                                  ? { ...msg, content: editInput.trim() }
                                  : msg
                              );
                              setMessages?.(updatedMessages);
                            }
                            setEditingMessageId(null);
                          }}
                          className="neo-button flex-1 flex items-center justify-center gap-2"
                        >
                          <Send className="w-4 h-4" />
                          <span>Send</span>
                        </button>
                      </div>
                    </div>
                  ) : (
                    <p className="text-lg whitespace-pre-wrap break-words">{message.content}</p>
                  )}
                  {message.sql && (
                    <div className="mt-4 bg-black text-white rounded-lg font-mono text-sm overflow-hidden">
                      <div className="flex flex-wrap items-center justify-between gap-2 mb-4 px-4 pt-4">
                        <div className="flex items-center gap-2 text-gray-400">
                          <span>Query:</span>
                        </div>
                        <div className="flex items-center">
                          {queryStates[message.id]?.isExecuting ? (
                            <button
                              onClick={() => {
                                // First clear the timeout to prevent the result from showing
                                if (queryTimeouts.current[message.id]) {
                                  clearTimeout(queryTimeouts.current[message.id]);
                                  delete queryTimeouts.current[message.id];
                                }
                                // Then set to example state
                                setQueryStates(prev => ({
                                  ...prev,
                                  [message.id]: { isExecuting: false, isExample: true }
                                }));
                                toast.error('Query cancelled', toastStyle);
                              }}
                              className="p-2 hover:bg-gray-800 rounded transition-colors text-red-500 hover:text-red-400"
                              title="Cancel query"
                            >
                              <XCircle className="w-4 h-4" />
                            </button>
                          ) : (
                            <button
                              onClick={() => {
                                // Clear existing timeout if any
                                if (queryTimeouts.current[message.id]) {
                                  clearTimeout(queryTimeouts.current[message.id]);
                                  delete queryTimeouts.current[message.id];
                                }
                                setQueryStates(prev => ({
                                  ...prev,
                                  [message.id]: { isExecuting: true, isExample: false }
                                }));
                                // Store timeout reference
                                queryTimeouts.current[message.id] = setTimeout(() => {
                                  // Only update state if the timeout wasn't cancelled
                                  if (queryTimeouts.current[message.id]) {
                                    setQueryStates(prev => ({
                                      ...prev,
                                      [message.id]: { isExecuting: false, isExample: false }
                                    }));
                                    delete queryTimeouts.current[message.id];
                                  }
                                }, 2000);
                              }}
                              className="p-2 hover:bg-gray-800 rounded transition-colors text-red-500 hover:text-red-400"
                              title="Run query"
                            >
                              <Play className="w-4 h-4" />
                            </button>
                          )}
                          <div className="w-px h-4 bg-gray-700 mx-2" />
                          <button
                            onClick={() => handleCopyToClipboard(message.sql || '')}
                            className="p-2 hover:bg-gray-800 rounded transition-colors text-white hover:text-gray-200"
                            title="Copy query"
                          >
                            <Copy className="w-4 h-4" />
                          </button>
                        </div>
                      </div>
                      <pre className="text-sm overflow-x-auto p-4 border-t border-gray-700">
                        <code className="whitespace-pre-wrap break-words">{message.sql}</code>
                      </pre>
                      {(message.result || message.error) && (
                        <div className="border-t border-gray-700 mt-2">
                          {queryStates[message.id]?.isExecuting ? (
                            <div className="flex items-center justify-center p-8">
                              <Loader className="w-8 h-8 animate-spin text-gray-400" />
                              <span className="ml-3 text-gray-400">Executing query...</span>
                            </div>
                          ) : (
                            <div className="mt-3 px-4 pt-4">
                              <div className="flex flex-wrap items-center justify-between gap-2 mb-4">
                                <div className="flex items-center gap-2 text-gray-400">
                                  {message.error ? (
                                    <span className="text-neo-error font-medium flex items-center gap-2">
                                      <AlertCircle className="w-4 h-4" />
                                      Error
                                    </span>
                                  ) : (
                                    <span>
                                      {queryStates[message.id]?.isExample === false ? 'Result:' : 'Example Result:'}
                                    </span>
                                  )}
                                  {message.executionTime && (
                                    <span className="text-xs bg-gray-800 px-2 py-1 rounded flex items-center gap-1">
                                      <Clock className="w-3 h-3" />
                                      {message.executionTime.toLocaleString()}ms
                                    </span>
                                  )}
                                </div>
                                {!message.error && <div className="flex gap-2">
                                  <div className="flex items-center">
                                    <button
                                      onClick={() => setViewMode('table')}
                                      className={`p-1 md:p-2 rounded ${viewMode === 'table' ? 'bg-gray-700' : 'hover:bg-gray-800'}`}
                                      title="Table view"
                                    >
                                      <Table className="w-4 h-4" />
                                    </button>
                                    <div className="w-px h-4 bg-gray-700 mx-2" />
                                    <button
                                      onClick={() => setViewMode('json')}
                                      className={`p-1 md:p-2 rounded ${viewMode === 'json' ? 'bg-gray-700' : 'hover:bg-gray-800'}`}
                                      title="JSON view"
                                    >
                                      <Braces className="w-4 h-4" />
                                    </button>
                                    <div className="w-px h-4 bg-gray-700 mx-2" />
                                    <button
                                      onClick={() => handleCopyToClipboard(JSON.stringify(message.result, null, 2))}
                                      className="p-2 hover:bg-gray-800 rounded text-white hover:text-gray-200"
                                      title="Copy result"
                                    >
                                      <Copy className="w-4 h-4" />
                                    </button>
                                    {queryStates[message.id]?.isExample === false && (
                                      <button
                                        onClick={() => {
                                          setQueryStates(prev => ({
                                            ...prev,
                                            [message.id]: { isExecuting: false, isExample: true }
                                          }));
                                          toast('Changes reverted', {
                                            ...toastStyle,
                                            icon: '↺',
                                          });
                                        }}
                                        className="p-2 hover:bg-gray-800 rounded text-yellow-400 hover:text-yellow-300"
                                        title="Rollback changes"
                                      >
                                        <History className="w-4 h-4" />
                                      </button>
                                    )}
                                  </div>
                                </div>}
                              </div>
                              {message.error ? (
                                <div className="bg-neo-error/10 text-neo-error p-4 rounded-lg mb-6">
                                  <div className="font-bold mb-2">{message.error.code}</div>
                                  <div className="mb-2">{message.error.message}</div>
                                  {message.error.details && (
                                    <div className="text-sm opacity-80 border-t border-neo-error/20 pt-2 mt-2">
                                      {message.error.details}
                                    </div>
                                  )}
                                </div>
                              ) : (
                                <div className="text-green-400 pb-6">
                                  {viewMode === 'table' ? (
                                    renderTableView(message.result || [])
                                  ) : (
                                    <pre className="overflow-x-auto whitespace-pre-wrap">
                                      {JSON.stringify(message.result, null, 2)}
                                    </pre>
                                  )}
                                </div>
                              )}
                            </div>
                          )}
                        </div>
                      )}
                    </div>
                  )}
                </div>
              </div>
            </div>
          ))}
        </div>
        <div ref={messagesEndRef} />

        {showScrollButton && (
          <button
            onClick={scrollToBottom}
            className="fixed bottom-24 right-8 p-3 bg-black text-white rounded-full shadow-lg hover:bg-gray-800 transition-all neo-border"
            title="Scroll to bottom"
          >
            <ArrowDown className="w-6 h-6" />
          </button>
        )}
      </div>

      <form
        onSubmit={handleSubmit}
        className={`
          fixed bottom-0 left-0 right-0 p-4 
          bg-white border-t-4 border-black 
          transition-all duration-300
          z-[10]
          ${isExpanded ? '' : 'md:left-[5rem]'}
        `}
      >
        <div className="max-w-5xl mx-auto flex gap-4">
          <textarea
            value={input}
            onChange={(e) => setInput(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Enter' && !e.shiftKey) {
                e.preventDefault();
                handleSubmit(e);
              }
            }}
            placeholder="Talk to your database..."
            className="
              neo-input 
              flex-1
              min-h-[52px]
              resize-y
              py-3
              px-4
              leading-normal
              whitespace-pre-wrap
            "
            rows={Math.min(
              Math.max(
                input.split('\n').length,
                Math.ceil(input.length / 50)
              ),
              5
            )}
            disabled={!isConnected}
          />
          <button
            type="submit"
            className="neo-button px-8 self-end"
            disabled={!isConnected}
          >
            <Send className="w-6 h-6" />
          </button>
        </div>
      </form>

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
    </div>

  );
}