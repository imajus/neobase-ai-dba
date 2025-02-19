import axios from 'axios';
import { Boxes, Database, LineChart, MessageSquare } from 'lucide-react';
import { useEffect, useState } from 'react';
import toast, { Toaster } from 'react-hot-toast';
import AuthForm from './components/auth/AuthForm';
import ChatWindow from './components/chat/ChatWindow';
import { Message } from './components/chat/types';
import StarUsButton from './components/common/StarUsButton';
import SuccessBanner from './components/common/SuccessBanner';
import Sidebar from './components/dashboard/Sidebar';
import ConnectionModal from './components/modals/ConnectionModal';
import mockMessages, { newMockMessage } from './data/mockMessages';
import authService from './services/authService';
import './services/axiosConfig';
import chatService from './services/chatService';
import { LoginFormData, SignupFormData } from './types/auth';
import { Chat, ChatsResponse, Connection } from './types/chat';

function App() {
  const [isAuthenticated, setIsAuthenticated] = useState(false);
  const [isLoading, setIsLoading] = useState(true);
  const [showConnectionModal, setShowConnectionModal] = useState(false);
  const [connections, setConnections] = useState<Chat[]>([]);
  const [isSidebarExpanded, setIsSidebarExpanded] = useState(true);
  const [selectedConnection, setSelectedConnection] = useState<Chat>();
  const [messages, setMessages] = useState<Message[]>(mockMessages);
  const [authError, setAuthError] = useState<string | null>(null);
  const [successMessage, setSuccessMessage] = useState<string | null>(null);
  const [chats, setChats] = useState<Chat[]>([]);
  const [isLoadingChats, setIsLoadingChats] = useState(false);

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
      setMessages(mockMessages);
    } catch (error: any) {
      console.error('Logout failed:', error);
      setIsAuthenticated(false);
    }
  };

  const handleClearChat = () => {
    setMessages([]);
  };

  const handleCloseConnection = () => {
    setSelectedConnection(undefined);
  };

  const handleDeleteConnection = async (id: string) => {
    try {
      // Remove from UI state
      setChats(prev => prev.filter(chat => chat.id !== id));

      // If the deleted chat was selected, clear the selection
      if (selectedConnection?.id === id) {
        setSelectedConnection(undefined);
        setMessages([]); // Clear messages if showing deleted chat
      }

      // Show success message
      setSuccessMessage('Connection deleted successfully');
    } catch (error: any) {
      console.error('Failed to delete connection:', error);
      toast.error(error.message, errorToast);
    }
  };

  const handleEditConnection = (id: string, data: Chat) => {
    setChats(prev => prev.map(chat => {
      if (chat.id === id) {
        return {
          ...chat,
          id: data.id,
          user_id: data.user_id,
          connection: data.connection,
          created_at: data.created_at,
          updated_at: data.updated_at,
        };
      }
      return chat;
    }));
    setSelectedConnection(data);
  };

  const generateAIResponse = async (userMessage: string) => {
    console.log('Generating AI response for:', userMessage);
    const aiMessageId = `ai-${Date.now()}`;
    const loadingSteps = [
      { text: "NeoBase is analyzing your request..", done: false },
      { text: "Fetching request relevant entities(tables, columns, etc.) from the database..", done: false },
      { text: "Generating an optimized query & example results for the request..", done: false },
      { text: "Analyzing the criticality of the query & if roll back is possible..", done: false }
    ];

    // Add initial loading message with first step only
    setMessages(prev => [...prev, {
      id: aiMessageId,
      type: 'ai',
      content: '',
      isLoading: true,
      loadingSteps: [loadingSteps[0]]
    }]);

    // Update steps one by one
    for (let i = 0; i < loadingSteps.length; i++) {
      await new Promise(resolve => setTimeout(resolve, 1500));
      setMessages(prev => prev.map(msg =>
        msg.id === aiMessageId ? {
          ...msg,
          loadingSteps: [
            ...loadingSteps.slice(0, i + 1).map(step => ({ ...step, done: true })),
            ...(i < loadingSteps.length - 1 ? [loadingSteps[i + 1]] : [])
          ]
        } : msg
      ));
    }

    // Mark last step as done and immediately start content streaming
    setMessages(prev => prev.map(msg =>
      msg.id === aiMessageId ? {
        ...msg,
        loadingSteps: loadingSteps.map(step => ({ ...step, done: true })),
        content: '',
        startStreaming: true
      } : msg
    ));

    // Start content streaming immediately
    const fullContent = newMockMessage.content;
    let currentContent = '';

    for (let i = 0; i < fullContent.length; i++) {
      await new Promise(resolve => setTimeout(resolve, 15 + Math.random() * 15));
      currentContent += fullContent[i];
      setMessages(prev => prev.map(msg =>
        msg.id === aiMessageId ? {
          ...msg,
          content: currentContent,
          loadingSteps: msg.loadingSteps?.map(step => ({
            ...step,
            transitioning: true
          }))
        } : msg
      ));
    }

    // Remove loading steps but keep isLoading true for query streaming
    setMessages(prev => prev.map(msg =>
      msg.id === aiMessageId ? {
        ...msg,
        loadingSteps: undefined,
        queries: []
      } : msg
    ));

    // Stream each query one by one
    for (let i = 0; i < (newMockMessage.queries?.length || 0); i++) {
      const query = newMockMessage.queries?.[i];
      if (!query) continue;

      // Stream query text
      let currentQuery = '';
      for (let j = 0; j < query.query.length; j++) {
        await new Promise(resolve => setTimeout(resolve, 10 + Math.random() * 10));
        currentQuery += query.query[j];
        setMessages(prev => prev.map(msg =>
          msg.id === aiMessageId ? {
            ...msg,
            queries: [
              ...(msg.queries || []).slice(0, i),
              { ...query, query: currentQuery, exampleResult: undefined },
              ...(msg.queries || []).slice(i + 1)
            ]
          } : msg
        ));
      }

      // Add example result gradually
      if (Array.isArray(query.exampleResult)) {
        for (let k = 0; k < query.exampleResult.length; k++) {
          await new Promise(resolve => setTimeout(resolve, 50));
          setMessages(prev => prev.map(msg =>
            msg.id === aiMessageId ? {
              ...msg,
              queries: msg.queries?.map((q, index) =>
                index === i ? {
                  ...q,
                  exampleResult: query.exampleResult?.slice(0, k + 1)
                } : q
              )
            } : msg
          ));
        }
      } else {
        await new Promise(resolve => setTimeout(resolve, 100));
        setMessages(prev => prev.map(msg =>
          msg.id === aiMessageId ? {
            ...msg,
            queries: msg.queries?.map((q, index) =>
              index === i ? { ...q, exampleResult: query.exampleResult } : q
            )
          } : msg
        ));
      }
    }

    // Finally remove loading state
    setMessages(prev => prev.map(msg =>
      msg.id === aiMessageId ? {
        ...msg,
        isLoading: false
      } : msg
    ));
  };

  if (isLoading) {
    return <div>Loading...</div>; // Or a proper loading component
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
        onSelectConnection={(id) => {
          setSelectedConnection(chats.find(chat => chat.id === id));
        }}
        onAddConnection={() => setShowConnectionModal(true)}
        onLogout={handleLogout}
        selectedConnection={selectedConnection}
        onDeleteConnection={handleDeleteConnection}
      />

      {selectedConnection ? (
        <ChatWindow
          chat={selectedConnection}
          isExpanded={isSidebarExpanded}
          messages={messages}
          setMessages={setMessages}
          onSendMessage={(message) => {
            const userMessageId = `user-${Date.now()}`;
            const newMessage = {
              id: userMessageId,
              type: 'user' as const,
              content: message,
            };
            setMessages(prev => [...prev, newMessage]);
            generateAIResponse(message);
          }}
          onClearChat={handleClearChat}
          onCloseConnection={handleCloseConnection}
          onEditConnection={handleEditConnection}
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

export default App;