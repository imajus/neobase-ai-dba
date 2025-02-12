import { Boxes, Database, LineChart, MessageSquare } from 'lucide-react';
import { useState } from 'react';
import { Toaster } from 'react-hot-toast';
import AuthForm from './components/auth/AuthForm';
import ChatWindow from './components/chat/ChatWindow';
import { Message } from './components/chat/types';
import StarUsButton from './components/common/StarUsButton';
import Sidebar from './components/dashboard/Sidebar';
import ConnectionModal, { ConnectionFormData } from './components/modals/ConnectionModal';
import mockConnections from './data/mockConnections';
import mockMessages, { newMockMessage } from './data/mockMessages';
import { LoginFormData, SignupFormData } from './types/auth';

function App() {
  const [isAuthenticated, setIsAuthenticated] = useState(false);
  const [showConnectionModal, setShowConnectionModal] = useState(false);
  const [connections, setConnections] = useState<ConnectionFormData[]>(mockConnections);
  const [isSidebarExpanded, setIsSidebarExpanded] = useState(true);
  const [selectedConnection, setSelectedConnection] = useState<ConnectionFormData>();
  const [messages, setMessages] = useState<Message[]>(mockMessages);
  const handleLogin = (data: LoginFormData) => {
    console.log('Login:', data);
    setIsAuthenticated(true);
  };

  const handleSignup = (data: SignupFormData) => {
    console.log('Signup:', data);
    setIsAuthenticated(true);
  };

  const handleAddConnection = () => {
    setShowConnectionModal(true);
  };

  const handleLogout = () => {
    setIsAuthenticated(false);
    setSelectedConnection(undefined);
    setMessages(mockMessages);
  };

  const handleClearChat = () => {
    setMessages([]);
  };

  const handleCloseConnection = () => {
    setSelectedConnection(undefined);
  };

  const handleDeleteConnection = (id: string) => {
    setConnections(prev => prev.filter(conn => conn.id !== id));
    if (selectedConnection?.id === id) {
      setSelectedConnection(undefined);
    }
  };

  const handleEditConnection = (id: string, data: ConnectionFormData) => {
    setConnections(prev => prev.map(conn => {
      if (conn.id === id) {
        return {
          ...conn,
          id: data.id,
          type: data.type,
          host: data.host,
          port: data.port,
          database: data.database,
          username: data.username,
          password: data.password,
        };
      }
      return conn;
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
        connections={connections}
        onSelectConnection={(id) => {
          setSelectedConnection(connections.find(conn => conn.id === id));
        }}
        onAddConnection={handleAddConnection}
        onLogout={handleLogout}
        selectedConnection={selectedConnection}
        onDeleteConnection={handleDeleteConnection}
      />

      {selectedConnection ? (
        <ChatWindow
          connection={selectedConnection}
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
                md:mt-16
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
              disabled={!connections.length}
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
              onClick={handleAddConnection}
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
              disabled={!connections.length}
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
              onClick={handleAddConnection}
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
          onSubmit={(data) => {
            const newConnection: ConnectionFormData = {
              id: Date.now().toString(),
              type: data.type,
              host: data.host,
              port: data.port,
              database: data.database,
              username: data.username,
              password: data.password,
            };
            setConnections(prev => [...prev, newConnection]);
            setShowConnectionModal(false);
          }}
        />
      )}
      <Toaster />
    </div>
  );
}

export default App;