import axios from 'axios';
import { EventSourcePolyfill } from 'event-source-polyfill';
import { Boxes, Database, LineChart, MessageSquare } from 'lucide-react';
import { useCallback, useEffect, useState } from 'react';
import toast, { Toaster } from 'react-hot-toast';
import AuthForm from './components/auth/AuthForm';
import ChatWindow from './components/chat/ChatWindow';
import { LoadingStep, Message, QueryResult } from './components/chat/types';
import StarUsButton from './components/common/StarUsButton';
import SuccessBanner from './components/common/SuccessBanner';
import Sidebar from './components/dashboard/Sidebar';
import ConnectionModal from './components/modals/ConnectionModal';
import { StreamProvider, useStream } from './contexts/StreamContext';
import authService from './services/authService';
import './services/axiosConfig';
import chatService from './services/chatService';
import { LoginFormData, SignupFormData } from './types/auth';
import { Chat, ChatsResponse, Connection } from './types/chat';
import { SendMessageResponse } from './types/messages';
import { StreamResponse } from './types/stream';


function AppContent() {
  const [isAuthenticated, setIsAuthenticated] = useState(false);
  const [isLoading, setIsLoading] = useState(true);
  const [showConnectionModal, setShowConnectionModal] = useState(false);
  const [connections, setConnections] = useState<Chat[]>([]);
  const [isSidebarExpanded, setIsSidebarExpanded] = useState(true);
  const [selectedConnection, setSelectedConnection] = useState<Chat>();
  const [messages, setMessages] = useState<Message[]>([]);
  const [authError, setAuthError] = useState<string | null>(null);
  const [successMessage, setSuccessMessage] = useState<string | null>(null);
  const [chats, setChats] = useState<Chat[]>([]);
  const [isLoadingChats, setIsLoadingChats] = useState(false);
  const [connectionStatuses, setConnectionStatuses] = useState<Record<string, boolean>>({});
  const [eventSource, setEventSource] = useState<EventSourcePolyfill | null>(null);
  const { streamId, setStreamId, generateStreamId } = useStream();
  const [isMessageSending, setIsMessageSending] = useState(false);
  const [temporaryMessage, setTemporaryMessage] = useState<Message | null>(null);

  // Check auth status on mount
  useEffect(() => {
    checkAuth();
  }, []);

  // First, update the toast configurations
  const toastStyle = {
    style: {
      background: '#000',
      color: '#fff',
      border: '4px solid #000',
      borderRadius: '12px',
      boxShadow: '4px 4px 0px 0px rgba(0,0,0,1)',
      padding: '12px 24px',
      fontSize: '16px',
      fontWeight: '500',
      zIndex: 9999,
    },
  } as const;


  const errorToast = {
    style: {
      ...toastStyle.style,
      background: '#ff4444',  // Red background for errors
      border: '4px solid #cc0000',
      color: '#fff',
      fontWeight: 'bold',
    },
    duration: 4000,
    icon: '⚠️',
  };

  const checkAuth = async () => {
    try {
      console.log("Starting auth check...");
      const isAuth = await authService.checkAuth();
      console.log("Auth check result:", isAuth);
      setIsAuthenticated(isAuth);
      setAuthError(null);
    } catch (error: any) {
      console.error('Auth check failed:', error);
      setIsAuthenticated(false);
      setAuthError(error.message);
      toast.error(error.message, errorToast);
    } finally {
      setIsLoading(false);
    }
  };

  // Add useEffect debug
  useEffect(() => {
    console.log("Auth state changed:", isAuthenticated);
  }, [isAuthenticated]);

  // Update useEffect to handle auth errors
  useEffect(() => {
    if (authError) {
      toast.error(authError, errorToast);
      setAuthError(null);
    }
  }, [authError]);

  // Load chats when authenticated
  useEffect(() => {
    const loadChats = async () => {
      if (!isAuthenticated) return;

      setIsLoadingChats(true);
      try {
        const response = await axios.get<ChatsResponse>(`${import.meta.env.VITE_API_URL}/chats`, {
          withCredentials: true,
          headers: {
            'Authorization': `Bearer ${localStorage.getItem('token')}`,
            'Accept': 'application/json',
            'Content-Type': 'application/json'
          }
        });
        console.log("Loaded chats:", response.data);
        if (response.data?.data?.chats) {
          setChats(response.data.data.chats);
        }
      } catch (error) {
        console.error("Failed to load chats:", error);
      } finally {
        setIsLoadingChats(false);
      }
    };

    loadChats();
  }, [isAuthenticated]);

  const handleLogin = async (data: LoginFormData) => {
    try {
      const response = await authService.login(data);
      console.log("handleLogin response", response);
      setIsAuthenticated(true);
      setSuccessMessage(`Welcome back, ${response.data.user.username}!`);
    } catch (error: any) {
      console.error("Login error:", error);
      throw error;
    }
  };

  const handleSignup = async (data: SignupFormData) => {
    try {
      const response = await authService.signup(data);
      console.log("handleSignup response", response);
      setIsAuthenticated(true);
      setSuccessMessage(`Welcome to NeoBase, ${response.data.user.username}!`);
    } catch (error: any) {
      console.error("Signup error:", error);
      throw error;
    }
  };

  const handleAddConnection = async (connection: Connection) => {
    try {
      const newChat = await chatService.createChat(connection);
      setChats(prev => [...prev, newChat]);
      setSuccessMessage('Connection added successfully!');
      setShowConnectionModal(false);
    } catch (error: any) {
      console.error('Failed to add connection:', error);
      toast.error(error.message, errorToast);
    }
  };

  const handleLogout = async () => {
    try {
      await authService.logout();
      setSuccessMessage('You\'ve been logged out!');
      setIsAuthenticated(false);
      setSelectedConnection(undefined);
      setMessages([]);
    } catch (error: any) {
      console.error('Logout failed:', error);
      setIsAuthenticated(false);
    }
  };

  const handleClearChat = async () => {
    // Make API call to clear chat
    try {
      await axios.delete(`${import.meta.env.VITE_API_URL}/chats/${selectedConnection?.id}/messages`, {
        withCredentials: true,
        headers: {
          'Authorization': `Bearer ${localStorage.getItem('token')}`
        }
      });
      setMessages([]);
    } catch (error: any) {
      console.error('Failed to clear chat:', error);
      toast.error(error.message, errorToast);
    }
  };

  const handleConnectionStatusChange = useCallback((chatId: string, isConnected: boolean, from: string) => {
    console.log('Connection status changed:', { chatId, isConnected, from });
    if (chatId && typeof isConnected === 'boolean') { // Strict type check
      setConnectionStatuses(prev => ({
        ...prev,
        [chatId]: isConnected
      }));
    }
  }, []);

  const handleCloseConnection = useCallback(async () => {
    if (eventSource) {
      console.log('Closing SSE connection');
      eventSource.close();
      setEventSource(null);
      // Disconnect from the connection
      await chatService.disconnectFromConnection(selectedConnection?.id || '');
      // Update connection status
      handleConnectionStatusChange(selectedConnection?.id || '', false, 'app-close-connection');
    }

    // Clear messages
    setMessages([]);

    // Clear connection status
    if (selectedConnection) {
      handleConnectionStatusChange(selectedConnection.id, false, 'app-close-connection');
    }

    // Clear messages and selected connection
    setMessages([]);
    setSelectedConnection(undefined);
  }, [eventSource, selectedConnection, handleConnectionStatusChange]);

  const handleDeleteConnection = async (id: string) => {
    try {
      // Remove from UI state
      setChats(prev => prev.filter(chat => chat.id !== id));

      // If the deleted chat was selected, clear the selection
      if (selectedConnection?.id === id) {
        setSelectedConnection(undefined);
        setMessages([]); // Clear messages if showing deleted chat
      }

      if (chats.length === 0) {
        setSelectedConnection(undefined);
      }
      // Show success message
      setSuccessMessage('Connection deleted successfully');
    } catch (error: any) {
      console.error('Failed to delete connection:', error);
      toast.error(error.message, errorToast);
    }
  };

  const handleEditConnection = async (id: string, data: Connection): Promise<{ success: boolean; error?: string }> => {
    const loadingToast = toast.loading('Updating connection...', {
      style: {
        background: '#000',
        color: '#fff',
        borderRadius: '12px',
        border: '4px solid #000',
      },
    });

    try {
      // Update the connection
      const response = await axios.put(
        `${import.meta.env.VITE_API_URL}/chats/${id}`,
        {
          connection: data
        },
        {
          withCredentials: true,
          headers: {
            'Content-Type': 'application/json',
            'Authorization': `Bearer ${localStorage.getItem('token')}`
          }
        }
      );


      if (response.data.success) {

        // First disconnect the current connection
        await axios.post(
          `${import.meta.env.VITE_API_URL}/chats/${id}/disconnect`,
          {
            stream_id: streamId
          },
          {
            withCredentials: true,
            headers: {
              'Authorization': `Bearer ${localStorage.getItem('token')}`
            }
          }
        );
        // Update local state
        setChats(prev => prev.map(chat => {
          if (chat.id === id) {
            return {
              ...chat,
              connection: data
            };
          }
          return chat;
        }));

        // Reconnect with new connection details
        await axios.post(
          `${import.meta.env.VITE_API_URL}/chats/${id}/connect`,
          {
            stream_id: streamId
          },
          {
            withCredentials: true,
            headers: {
              'Authorization': `Bearer ${localStorage.getItem('token')}`
            }
          }
        );

        // Update connection status
        handleConnectionStatusChange(id, true, 'edit-connection');

        // Dismiss loading toast and show success
        toast.dismiss(loadingToast);
        toast.success('Connection updated & reconnected', {
          style: {
            background: '#000',
            color: '#fff',
            borderRadius: '12px',
          },
        });

        return { success: true };
      }

      throw new Error('Failed to update connection');
    } catch (error: any) {
      console.error('Failed to update connection:', error);
      toast.dismiss(loadingToast);
      toast.error(error.response?.data?.error || 'Failed to update connection', {
        style: {
          background: '#ff4444',
          color: '#fff',
          border: '4px solid #cc0000',
          borderRadius: '12px',
          boxShadow: '4px 4px 0px 0px rgba(0,0,0,1)',
          padding: '12px 24px',
        }
      });


      return {
        success: false,
        error: error.response?.data?.error || 'Failed to update connection'
      };
    }
  };

  // const generateAIResponse = async (userMessage: string) => {
  //   console.log('Generating AI response for:', userMessage);
  //   const aiMessageId = `ai-${Date.now()}`;
  //   const loadingSteps = [
  //     { text: "NeoBase is analyzing your request..", done: false },
  //     { text: "Fetching request relevant entities(tables, columns, etc.) from the database..", done: false },
  //     { text: "Generating an optimized query & example results for the request..", done: false },
  //     { text: "Analyzing the criticality of the query & if roll back is possible..", done: false }
  //   ];

  //   // Add initial loading message with first step only
  //   setMessages(prev => [...prev, {
  //     id: aiMessageId,
  //     type: 'ai',
  //     content: '',
  //     isLoading: true,
  //     loadingSteps: [loadingSteps[0]]
  //   }]);

  //   // Update steps one by one
  //   for (let i = 0; i < loadingSteps.length; i++) {
  //     await new Promise(resolve => setTimeout(resolve, 1500));
  //     setMessages(prev => prev.map(msg =>
  //       msg.id === aiMessageId ? {
  //         ...msg,
  //         loadingSteps: [
  //           ...loadingSteps.slice(0, i + 1).map(step => ({ ...step, done: true })),
  //           ...(i < loadingSteps.length - 1 ? [loadingSteps[i + 1]] : [])
  //         ]
  //       } : msg
  //     ));
  //   }

  //   // Mark last step as done and immediately start content streaming
  //   setMessages(prev => prev.map(msg =>
  //     msg.id === aiMessageId ? {
  //       ...msg,
  //       loadingSteps: loadingSteps.map(step => ({ ...step, done: true })),
  //       content: '',
  //       startStreaming: true
  //     } : msg
  //   ));

  //   // Start content streaming immediately
  //   const fullContent = newMockMessage.content;
  //   let currentContent = '';

  //   for (let i = 0; i < fullContent.length; i++) {
  //     await new Promise(resolve => setTimeout(resolve, 15 + Math.random() * 15));
  //     currentContent += fullContent[i];
  //     setMessages(prev => prev.map(msg =>
  //       msg.id === aiMessageId ? {
  //         ...msg,
  //         content: currentContent,
  //         loadingSteps: msg.loadingSteps?.map(step => ({
  //           ...step,
  //           transitioning: true
  //         }))
  //       } : msg
  //     ));
  //   }

  //   // Remove loading steps but keep isLoading true for query streaming
  //   setMessages(prev => prev.map(msg =>
  //     msg.id === aiMessageId ? {
  //       ...msg,
  //       loadingSteps: undefined,
  //       queries: []
  //     } : msg
  //   ));

  //   // Stream each query one by one
  //   for (let i = 0; i < (newMockMessage.queries?.length || 0); i++) {
  //     const query = newMockMessage.queries?.[i];
  //     if (!query) continue;

  //     // Stream query text
  //     let currentQuery = '';
  //     for (let j = 0; j < query.query.length; j++) {
  //       await new Promise(resolve => setTimeout(resolve, 10 + Math.random() * 10));
  //       currentQuery += query.query[j];
  //       setMessages(prev => prev.map(msg =>
  //         msg.id === aiMessageId ? {
  //           ...msg,
  //           queries: [
  //             ...(msg.queries || []).slice(0, i),
  //             { ...query, query: currentQuery, exampleResult: undefined },
  //             ...(msg.queries || []).slice(i + 1)
  //           ]
  //         } : msg
  //       ));
  //     }

  //     // Add example result gradually
  //     if (Array.isArray(query.exampleResult)) {
  //       for (let k = 0; k < query.exampleResult.length; k++) {
  //         await new Promise(resolve => setTimeout(resolve, 50));
  //         setMessages(prev => prev.map(msg =>
  //           msg.id === aiMessageId ? {
  //             ...msg,
  //             queries: msg.queries?.map((q, index) =>
  //               index === i ? {
  //                 ...q,
  //                 exampleResult: query.exampleResult?.slice(0, k + 1)
  //               } : q
  //             )
  //           } : msg
  //         ));
  //       }
  //     } else {
  //       await new Promise(resolve => setTimeout(resolve, 100));
  //       setMessages(prev => prev.map(msg =>
  //         msg.id === aiMessageId ? {
  //           ...msg,
  //           queries: msg.queries?.map((q, index) =>
  //             index === i ? { ...q, exampleResult: query.exampleResult } : q
  //           )
  //         } : msg
  //       ));
  //     }
  //   }

  //   // Finally remove loading state
  //   setMessages(prev => prev.map(msg =>
  //     msg.id === aiMessageId ? {
  //       ...msg,
  //       isLoading: false
  //     } : msg
  //   ));
  // };

  // Clear connection status when connection is deselected
  useEffect(() => {
    if (!selectedConnection) {
      setConnectionStatuses({});
    }
  }, [selectedConnection]);

  const handleSelectConnection = useCallback(async (id: string) => {
    console.log('handleSelectConnection happened in app.tsx', { id });
    const connection = chats.find(c => c.id === id);
    if (connection) {
      console.log('connection found', { connection });
      setSelectedConnection(connection);

      // Check if the connection is already connected
      const isConnected = connectionStatuses[id];
      if (isConnected) {
        handleConnectionStatusChange(id, true, 'app-select-connection');
      } else {
        // Make api call to to check connection status
        const connectionStatus = await chatService.checkConnectionStatus(id);
        console.log('connectionStatus in handleSelectConnection', { connectionStatus });
        if (connectionStatus) {
          handleConnectionStatusChange(id, true, 'app-select-connection');
        } else {
          console.log('connectionStatus is false, connecting to the connection');
          // Make api call to connect to the connection
          await chatService.connectToConnection(id, streamId || '');
          handleConnectionStatusChange(id, true, 'app-select-connection');
        }
      }

      // Check eventsource state
      console.log('eventSource?.readyState', eventSource?.readyState);
      if (eventSource?.readyState === EventSource.OPEN) {
        console.log('eventSource is open');
      } else {
        console.log('eventSource is not open, setting up');
        await setupSSEConnection(id);
      }
    }
  }, [chats, connectionStatuses, handleConnectionStatusChange]);

  // Add onClose handler for EventSource
  useEffect(() => {
    if (!eventSource) return;

    const handleClose = () => {
      console.log('SSE connection closed');
      // Update connection status
      if (selectedConnection) {
        handleConnectionStatusChange(selectedConnection.id, false, 'sse-close');
      }
      setEventSource(null);
    };


  }, [eventSource, selectedConnection, handleConnectionStatusChange]);

  // Update setupSSEConnection to include onclose
  const setupSSEConnection = useCallback(async (chatId: string) => {
    try {
      // Close existing SSE connection if any
      if (eventSource) {
        eventSource.close();
        setEventSource(null);
      }

      const newStreamId = generateStreamId();
      setStreamId(newStreamId);

      // Create and setup new SSE connection
      const sse = new EventSourcePolyfill(
        `${import.meta.env.VITE_API_URL}/chats/${chatId}/stream?stream_id=${newStreamId}`,
        {
          withCredentials: true,
          headers: {
            'Authorization': `Bearer ${localStorage.getItem('token')}`
          }
        }
      );

      // Setup SSE event handlers
      sse.onmessage = (event) => {
        try {
          const data = JSON.parse(event.data);
          console.log('SSE message:', data);

          if (data.event === 'db-connected') {
            handleConnectionStatusChange(chatId, true, 'app-sse-connection');
          } else if (data.event === 'db-disconnected') {
            handleConnectionStatusChange(chatId, false, 'app-sse-connection');
          }
        } catch (error) {
          console.error('Failed to parse SSE message:', error);
        }
      };

      sse.onerror = (e: any) => {
        console.log('SSE connection error', e);
        handleConnectionStatusChange(chatId, false, 'sse-close');
        setEventSource(null);
      };

      setEventSource(sse);
      return newStreamId;
    } catch (error) {
      console.error('Failed to setup SSE connection:', error);
      throw error;
    }
  }, [eventSource, generateStreamId, setStreamId, handleConnectionStatusChange]);

  // Cleanup SSE on unmount or connection change
  useEffect(() => {
    return () => {
      if (eventSource) {
        eventSource.close();
        setEventSource(null);
      }
    };
  }, [eventSource]);

  const handleCancelStream = async () => {
    if (!selectedConnection?.id || !streamId) return;
    try {
      console.log('handleCancelStream -> streamId', streamId);
      await axios.post(
        `${import.meta.env.VITE_API_URL}/chats/${selectedConnection.id}/stream/cancel?stream_id=${streamId}`,
        {},
        {
          withCredentials: true,
          headers: {
            'Authorization': `Bearer ${localStorage.getItem('token')}`
          }
        }
      );

      // Remove temporary streaming message
      setMessages(prev => {
        return prev.filter(msg => !(msg.is_streaming && msg.id === 'temp'));
      });


    } catch (error) {
      console.error('Failed to cancel stream:', error);
    }
  };

  // Add helper function for typing animation
  const animateTyping = async (text: string, messageId: string) => {
    const words = text.split(' ');
    for (const word of words) {
      await new Promise(resolve => setTimeout(resolve, 15 + Math.random() * 15));
      setMessages(prev => {
        const [lastMessage, ...rest] = prev;
        if (lastMessage?.id === messageId) {
          return [
            {
              ...lastMessage,
              content: lastMessage.content + (lastMessage.content ? ' ' : '') + word,
            },
            ...rest
          ];
        }
        return prev;
      });
    }
  };

  const handleSendMessage = async (content: string) => {
    if (!selectedConnection?.id || !streamId || isMessageSending) return;

    try {
      console.log('handleSendMessage -> content', content);
      console.log('handleSendMessage -> streamId', streamId);
      // Check if the eventSource is open
      console.log('eventSource?.readyState', eventSource?.readyState);
      if (eventSource?.readyState === EventSource.OPEN) {
        console.log('EventSource is open');
      } else {
        console.log('EventSource is not open');
        // Push an error message to the messages array
        const errorMsg: Message = {
          id: `error-${Date.now()}`,
          type: 'assistant',
          content: '',  // Start empty for animation
          queries: [],
          is_loading: false,
          is_streaming: true
        };

        setMessages(prev => [errorMsg, ...prev]);

        // Animate error message
        await animateTyping(
          '❌ Error: SSE connection is not open. We\'ve automatically reconnected. Please try again.',
          errorMsg.id
        );

        // Set final state
        setMessages(prev => {
          const [lastMessage, ...rest] = prev;
          if (lastMessage?.id === errorMsg.id) {
            return [{ ...lastMessage, is_streaming: false }, ...rest];
          }
          return prev;
        });

        await setupSSEConnection(selectedConnection.id);
        return;
      }

      const response = await axios.post<SendMessageResponse>(
        `${import.meta.env.VITE_API_URL}/chats/${selectedConnection.id}/messages`,
        {
          stream_id: streamId,
          content: content
        },
        {
          withCredentials: true,
          headers: {
            'Content-Type': 'application/json',
            'Authorization': `Bearer ${localStorage.getItem('token')}`
          }
        }
      );

      if (response.data.success) {
        const userMessage: Message = {
          id: response.data.data.id,
          type: 'user',
          content: response.data.data.content,
          is_loading: false,
          queries: [],
          is_streaming: false
        };


        setMessages(prev => [userMessage, ...prev]);

        console.log('ai-response-step -> creating new temp message');
        const tempMsg: Message = {
          id: `temp`,
          type: 'assistant',
          content: '',
          queries: [],
          is_loading: true,
          loading_steps: [{ text: 'NeoBase is analyzing your request..', done: false }],
          is_streaming: true
        };

        // Update messages first to remove any existing streaming messages
        setMessages(prev => {
          const withoutTemp = prev.filter(msg => !msg.is_streaming);
          return [tempMsg, ...withoutTemp];
        });

        setTemporaryMessage(tempMsg);
      }
    } catch (error) {
      console.error('Failed to send message:', error);
      toast.error('Failed to send message', errorToast);
    }
  };

  // Update SSE handling
  useEffect(() => {
    if (!eventSource) return;

    const handleSSEMessage = async function (this: EventSource, e: any) {
      try {
        console.log('handleSSEMessage -> msg', e);
        const response: StreamResponse = JSON.parse(e.data);
        console.log('handleSSEMessage -> response', response);

        switch (response.event) {
          case 'db-connected':
            console.log('db-connected -> response', response);
            if (selectedConnection) {
              handleConnectionStatusChange(selectedConnection.id, true, 'app-sse-connection');
            }

            break;
          case 'db-disconnected':
            console.log('db-disconnected -> response', response);
            if (selectedConnection) {
              handleConnectionStatusChange(selectedConnection.id, false, 'app-sse-connection');
            }
            break;
          case 'ai-response-step':
            // Set default of 500 ms delay for first step
            await new Promise(resolve => setTimeout(resolve, 500));

            if (!temporaryMessage) {
              console.log('ai-response-step -> creating new temp message');
            } else {
              console.log('ai-response-step -> updating existing temp message');
              // Update the existing message with new step
              setMessages(prev => {
                // Find the streaming message
                const streamingMessage = prev.find(msg => msg.is_streaming);
                if (!streamingMessage) return prev;

                // No need to update the message if the step is NeoBase is analyzing your request..
                if (streamingMessage.loading_steps && streamingMessage.loading_steps.length > 0 && response.data === 'NeoBase is analyzing your request..') {
                  return prev;
                }
                // Create updated message with new step
                const updatedMessage = {
                  ...streamingMessage,
                  loading_steps: [
                    ...(streamingMessage.loading_steps || []).map((step: LoadingStep) => ({ ...step, done: true })),
                    { text: response.data, done: false }
                  ]
                };

                // Replace the streaming message in the array
                return prev.map(msg =>
                  msg.id === streamingMessage.id ? updatedMessage : msg
                );
              });
            }
            break;

          case 'ai-response':
            if (response.data) {
              console.log('ai-response -> response.data', response.data);

              // Create base message with empty loading steps
              const baseMessage: Message = {
                id: response.data.id,
                type: 'assistant' as const,
                content: '',
                queries: response.data.queries || [],
                is_loading: false,
                loading_steps: [], // Clear loading steps for final message
                is_streaming: true
              };

              setMessages(prev => {
                const withoutTemp = prev.filter(msg => !msg.is_streaming);
                console.log('ai-response -> withoutTemp', withoutTemp);
                return [baseMessage, ...withoutTemp];
              });

              // Animate both content and queries with slower speed
              const finalWords = response.data.content.split(' ');
              for (const word of finalWords) {
                await new Promise(resolve => setTimeout(resolve, 50)); // Increased delay to 50ms
                setMessages(prev => {
                  const [lastMessage, ...rest] = prev;
                  if (lastMessage?.id === response.data.id) {
                    return [
                      {
                        ...lastMessage,
                        content: lastMessage.content + (lastMessage.content ? ' ' : '') + word,
                      },
                      ...rest
                    ];
                  }
                  return prev;
                });
              }

              // Slower animation for queries too
              if (response.data.queries && response.data.queries.length > 0) {
                for (const query of response.data.queries) {
                  const queryWords = query.query.split(' ');
                  for (const word of queryWords) {
                    await new Promise(resolve => setTimeout(resolve, 40)); // Increased delay to 40ms
                    setMessages(prev => {
                      const [lastMessage, ...rest] = prev;
                      if (lastMessage?.id === response.data.id) {
                        const updatedQueries = [...(lastMessage.queries || [])];
                        const queryIndex = updatedQueries.findIndex(q => q.id === query.id);
                        if (queryIndex !== -1) {
                          updatedQueries[queryIndex] = {
                            ...updatedQueries[queryIndex],
                            query: updatedQueries[queryIndex].query + (updatedQueries[queryIndex].query ? ' ' : '') + word
                          };
                        }
                        return [
                          {
                            ...lastMessage,
                            queries: updatedQueries
                          },
                          ...rest
                        ];
                      }
                      return prev;
                    });
                  }
                }
              }

              // Set final state
              setMessages(prev => {
                const [lastMessage, ...rest] = prev;
                if (lastMessage?.id === response.data.id) {
                  return [
                    {
                      ...lastMessage,
                      is_streaming: false
                    },
                    ...rest
                  ];
                }
                return prev;
              });
            }
            setTemporaryMessage(null);
            break;

          case 'ai-response-error':
            // Show error message instead of temporary message
            setMessages(prev => {
              const withoutTemp = prev.filter(msg => !msg.is_streaming);
              return [{
                id: `error-${Date.now()}`,
                type: 'assistant',
                content: `${typeof response.data === 'object' ? response.data.error : response.data}`, // Handle both string and object errors
                queries: [],
                is_loading: false,
                loading_steps: [],
                is_streaming: false
              }, ...withoutTemp];
            });
            setTemporaryMessage(null);

            break;

          case 'response-cancelled':

            // Remove temporary streaming message
            setMessages(prev => {
              return prev.filter(msg => !(msg.is_streaming && msg.id === 'temp'));
            });

            const cancelMsg: Message = {
              id: `cancelled-${Date.now()}`,
              type: 'assistant',
              content: '',  // Start empty for animation
              queries: [],
              is_loading: false,
              loading_steps: [], // Clear loading steps
              is_streaming: false // Set to false immediately
            };

            // Add cancel message
            setMessages(prev => {
              const withoutTemp = prev.filter(msg => !msg.is_streaming);
              return [cancelMsg, ...withoutTemp];
            });

            // Animate cancel message
            await animateTyping(response.data, cancelMsg.id);

            // Clear temporary message state
            setTemporaryMessage(null);

            // Set streaming to false for all messages
            setMessages(prev =>
              prev.map(msg => ({
                ...msg,
                is_streaming: false
              }))
            );
            break;

          case 'query-executed':
            setMessages(prev => prev.map(msg => {
              if (msg.id === response.data.message_id) {
                return {
                  ...msg,
                  queries: msg.queries?.map(q => {
                    if (q.id === response.data.query_id) {
                      return {
                        ...q,
                        is_executed: true,
                        is_rolled_back: false,
                        execution_time: response.data.execution_time,
                        execution_result: response.data.execution_result,
                        error: undefined // Clear any existing error
                      } as QueryResult;
                    }
                    return q;
                  })
                };
              }
              return msg;
            }));
            // Show success toast
            toast('Query executed!', {
              ...toastStyle,
              icon: '✅',
            });
            break;

          case 'query-execution-failed':
            setMessages(prev => prev.map(msg => {
              if (msg.id === response.data.message_id) {
                return {
                  ...msg,
                  queries: msg.queries?.map(q => {
                    if (q.id === response.data.query_id) {
                      return {
                        ...q,
                        is_executed: false,
                        is_rolled_back: false,
                        execution_result: undefined,
                        error: response.data.error
                      } as QueryResult;
                    }
                    return q;
                  })
                };
              }
              return msg;
            }));
            break;

          case 'rollback-executed':
            setMessages(prev => prev.map(msg => {
              if (msg.id === response.data.message_id) {
                return {
                  ...msg,
                  queries: msg.queries?.map(q => {
                    if (q.id === response.data.query_id) {
                      return {
                        ...q,
                        is_executed: true,
                        is_rolled_back: true,
                        execution_time: response.data.execution_time,
                        execution_result: response.data.execution_result,
                        error: undefined
                      } as QueryResult;
                    }
                    return q;
                  })
                };
              }
              return msg;
            }));
            toast('Changes reverted', {
              ...toastStyle,
              icon: '↺',
            });
            break;

          case 'rollback-query-failed':
            setMessages(prev => prev.map(msg => {
              if (msg.id === response.data.message_id) {
                return {
                  ...msg,
                  queries: msg.queries?.map(q => {
                    if (q.id === response.data.query_id) {
                      return {
                        ...q,
                        is_rolled_back: false,
                        error: response.data.error
                      } as QueryResult;
                    }
                    return q;
                  })
                };
              }
              return msg;
            }));
            toast.error(`Rollback failed: ${response.data.error.message}`, errorToast);
            break;
        }
      } catch (error) {
        console.error('Failed to parse SSE message:', error);
      }
    };

    eventSource.onmessage = handleSSEMessage

    return () => {
      eventSource.onmessage = null;
    };
  }, [eventSource, temporaryMessage, selectedConnection?.id, streamId]);

  if (isLoading) {
    return <div className="flex items-center justify-center bg-white h-screen">Loading...</div>; // Or a proper loading component
  }

  if (!isAuthenticated) {
    return (
      <>
        <AuthForm onLogin={handleLogin} onSignup={handleSignup} />
        <StarUsButton />
      </>
    );
  }

  return (
    <div className="flex flex-col md:flex-row bg-[#FFDB58]/10 min-h-screen">
      {/* Mobile header with StarUsButton */}
      <div className="fixed top-0 left-0 right-0 h-16 bg-white border-b-4 border-black md:hidden z-50 flex items-center justify-between px-4">
        <div className="flex items-center gap-2">
          <Boxes className="w-8 h-8" />
          <h1 className="text-2xl font-bold">NeoBase</h1>
        </div>
        {/* Show StarUsButton on mobile header */}
        <div className="block md:hidden">
          <StarUsButton isMobile className="scale-90" />
        </div>
      </div>

      {/* Desktop StarUsButton */}
      <div className="hidden md:block">
        <StarUsButton />
      </div>

      <Sidebar
        isExpanded={isSidebarExpanded}
        onToggleExpand={() => setIsSidebarExpanded(!isSidebarExpanded)}
        connections={chats}
        isLoadingConnections={isLoadingChats}
        onSelectConnection={handleSelectConnection}
        onAddConnection={() => setShowConnectionModal(true)}
        onLogout={handleLogout}
        selectedConnection={selectedConnection}
        onDeleteConnection={handleDeleteConnection}
        onConnectionStatusChange={handleConnectionStatusChange}
        setupSSEConnection={setupSSEConnection}
        eventSource={eventSource}
      />

      {selectedConnection ? (
        <ChatWindow
          chat={selectedConnection}
          isExpanded={isSidebarExpanded}
          messages={messages}
          setMessages={setMessages}
          onSendMessage={handleSendMessage}
          onClearChat={handleClearChat}
          onCloseConnection={handleCloseConnection}
          onEditConnection={handleEditConnection}
          onConnectionStatusChange={handleConnectionStatusChange}
          isConnected={!!connectionStatuses[selectedConnection.id]}
          onCancelStream={handleCancelStream}
        />
      ) : (
        <div className={`
                flex-1 
                flex 
                flex-col 
                items-center 
                justify-center
                p-8 
                mt-24
                md:mt-12
                min-h-[calc(100vh-4rem)] 
                transition-all 
                duration-300 
                ${isSidebarExpanded ? 'md:ml-80' : 'md:ml-20'}
            `}>
          {/* Welcome Section */}
          <div className="w-full max-w-4xl mx-auto text-center mb-12">
            <h1 className="text-5xl font-bold mb-4">
              Welcome to NeoBase
            </h1>
            <p className="text-xl text-gray-600 mb-2 max-w-2xl mx-auto">
              Open-source AI-powered engine for seamless database interactions.
              <br />
              From SQL to NoSQL, explore and analyze your data through natural conversations.
            </p>
          </div>

          {/* Features Cards */}
          <div className="w-full max-w-4xl mx-auto grid md:grid-cols-3 gap-6 mb-12">
            <button
              onClick={() => setSelectedConnection(connections[0])}
              className="
                            neo-border 
                            bg-white 
                            p-6 
                            rounded-lg
                            text-left
                            transition-all
                            duration-300
                            hover:-translate-y-1
                            hover:shadow-lg
                            hover:bg-[#FFDB58]/5
                            active:translate-y-0
                            disabled:opacity-50
                            disabled:cursor-not-allowed
                        "
            >
              <div className="w-12 h-12 bg-[#FFDB58]/20 rounded-lg flex items-center justify-center mb-4">
                <MessageSquare className="w-6 h-6 text-black" />
              </div>
              <h3 className="text-lg font-bold mb-2">
                Natural Language Queries
              </h3>
              <p className="text-gray-600">
                Talk to your database in plain English. NeoBase translates your questions into database queries automatically.
              </p>
            </button>

            <button
              onClick={() => setShowConnectionModal(true)}
              className="
                            neo-border 
                            bg-white 
                            p-6 
                            rounded-lg
                            text-left
                            transition-all
                            duration-300
                            hover:-translate-y-1
                            hover:shadow-lg
                            hover:bg-[#FFDB58]/5
                            active:translate-y-0
                        "
            >
              <div className="w-12 h-12 bg-[#FFDB58]/20 rounded-lg flex items-center justify-center mb-4">
                <Database className="w-6 h-6 text-black" />
              </div>
              <h3 className="text-lg font-bold mb-2">
                Multi-Database Support
              </h3>
              <p className="text-gray-600">
                Connect to PostgreSQL, MySQL, MongoDB, Redis, and more. One interface for all your databases.
              </p>
            </button>

            <button
              onClick={() => setSelectedConnection(connections[0])}
              className="
                            neo-border 
                            bg-white 
                            p-6 
                            rounded-lg
                            text-left
                            transition-all
                            duration-300
                            hover:-translate-y-1
                            hover:shadow-lg
                            hover:bg-[#FFDB58]/5
                            active:translate-y-0
                            disabled:opacity-50
                            disabled:cursor-not-allowed
                        "
            >
              <div className="w-12 h-12 bg-[#FFDB58]/20 rounded-lg flex items-center justify-center mb-4">
                <LineChart className="w-6 h-6 text-black" />
              </div>
              <h3 className="text-lg font-bold mb-2">
                Visual Results
              </h3>
              <p className="text-gray-600">
                View your data in tables or JSON format. Execute queries and see results in real-time.
              </p>
            </button>
          </div>

          {/* CTA Section */}
          <div className="text-center">
            <button
              onClick={() => setShowConnectionModal(true)}
              className="neo-button text-lg px-8 py-4 mb-4"
            >
              Create New Connection
            </button>
            <p className="text-gray-600">
              or select an existing one from the sidebar to begin
            </p>
          </div>
        </div>
      )}

      {showConnectionModal && (
        <ConnectionModal
          onClose={() => setShowConnectionModal(false)}
          onSubmit={handleAddConnection}
        />
      )}
      <Toaster
        position="bottom-center"
        reverseOrder={false}
        gutter={8}
        containerClassName="!fixed"
        containerStyle={{
          zIndex: 9999,
          bottom: '2rem'
        }}
        toastOptions={{
          success: {
            style: toastStyle.style,
            duration: 2000,
            icon: '👋',
          },
          error: {
            style: {
              ...toastStyle.style,
              background: '#ff4444',
              border: '4px solid #cc0000',
              color: '#fff',
              fontWeight: 'bold',
            },
            duration: 4000,
            icon: '⚠️',
          },
        }}
      />
      {successMessage && (
        <SuccessBanner
          message={successMessage}
          onClose={() => setSuccessMessage(null)}
        />
      )}
    </div>
  );
}

function App() {
  return (
    <StreamProvider>
      <AppContent />
    </StreamProvider>
  );
}

export default App;