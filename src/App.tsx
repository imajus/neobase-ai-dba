import { Boxes } from 'lucide-react';
import { useState } from 'react';
import { Toaster } from 'react-hot-toast';
import AuthForm from './components/auth/AuthForm';
import ChatWindow, { Message } from './components/chat/ChatWindow';
import Sidebar, { Connection } from './components/dashboard/Sidebar';
import ConnectionModal from './components/modals/ConnectionModal';
import { LoginFormData, SignupFormData } from './types/auth';

// Mock data for demonstration
const mockConnections = [
  { id: '1', name: 'Production DB', type: 'postgresql' as const },
  { id: '2', name: 'Development DB', type: 'mysql' as const },
];

const mockMessages = [
  {
    id: 'm1',
    type: 'user' as const,
    content: 'Show me all active users in the database',
  },
  {
    id: 'm2',
    type: 'ai' as const,
    content: 'Here are all users in the database:',
    sql: 'SELECT * FROM users WHERE active = true ORDER BY last_login DESC LIMIT 10;',
    executionTime: 42,
    result: [
      { id: 1, email: 'john@example.com', last_login: '2024-03-10T15:30:00Z', active: true },
      { id: 2, email: 'sarah@example.com', last_login: '2024-03-10T14:45:00Z', active: true },
      { id: 3, email: 'mike@example.com', last_login: '2024-03-10T13:20:00Z', active: true }
    ]
  },
  {
    id: 'm3',
    type: 'user' as const,
    content: 'How many orders were placed in the last 24 hours?',
  },
  {
    id: 'm4',
    type: 'ai' as const,
    content: 'I\'ll check the orders from the last 24 hours:',
    sql: 'SELECT COUNT(*) as order_count FROM orders WHERE created_at >= NOW() - INTERVAL \'24 hours\';',
    executionTime: 156,
    result: [
      { order_count: 157 }
    ]
  },
  {
    id: 'm5',
    type: 'user' as const,
    content: 'What are our top 5 selling products this month?',
  },
  {
    id: 'm6',
    type: 'ai' as const,
    content: 'Here are the top 5 selling products for this month:',
    sql: `SELECT
  p.name, 
  SUM(oi.quantity) as total_sold,
  SUM(oi.quantity * oi.price) as revenue
FROM order_items oi
JOIN products p ON p.id = oi.product_id
WHERE DATE_TRUNC('month', oi.created_at) = DATE_TRUNC('month', CURRENT_DATE)
GROUP BY p.id, p.name
ORDER BY total_sold DESC
LIMIT 5;`,
    executionTime: 234,
    result: [
      { name: 'Wireless Earbuds Pro', total_sold: 245, revenue: 24500.00 },
      { name: 'Smart Watch X3', total_sold: 189, revenue: 37800.00 },
      { name: 'Gaming Mouse', total_sold: 156, revenue: 7800.00 },
      { name: 'Mechanical Keyboard', total_sold: 134, revenue: 13400.00 },
      { name: 'USB-C Hub', total_sold: 98, revenue: 2940.00 }
    ]
  },
  {
    id: 'm7',
    type: 'user' as const,
    content: 'Show me all transactions with invalid amounts',
  },
  {
    id: 'm8',
    type: 'ai' as const,
    content: 'Here are the top 5 selling products for this month:',
    sql: `SELECT
  t.id,
  t.amount,
  t.created_at
FROM transactions t
WHERE t.amount <= 0
  OR t.amount IS NULL
ORDER BY t.created_at DESC;`,
    executionTime: 234,
    error: {
      code: 'ER_NO_SUCH_TABLE',
      message: 'Table \'myapp.transactions\' doesn\'t exist',
      details: 'The table "transactions" does not exist in the current database. Please make sure the table exists and you have the necessary permissions to access it.'
    }
  },
];

function App() {
  const [isAuthenticated, setIsAuthenticated] = useState(false);
  const [showConnectionModal, setShowConnectionModal] = useState(false);
  const [connections, setConnections] = useState<Connection[]>(mockConnections);
  const [isSidebarExpanded, setIsSidebarExpanded] = useState(true);
  const [selectedConnectionId, setSelectedConnectionId] = useState<string>();
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
    setSelectedConnectionId(undefined);
    setMessages(mockMessages);
  };

  const handleClearChat = () => {
    setMessages([]);
  };

  const handleCloseConnection = () => {
    setSelectedConnectionId(undefined);
  };

  const handleDeleteConnection = (id: string) => {
    setConnections(prev => prev.filter(conn => conn.id !== id));
    if (selectedConnectionId === id) {
      setSelectedConnectionId(undefined);
    }
  };

  const generateAIResponse = async (userMessage: string) => {
    console.log('Generating AI response for:', userMessage);
    // First, add a loading message
    const loadingMessage = {
      id: `ai-${Date.now()}`,
      type: 'ai' as const,
      content: '',
      isLoading: true
    };
    setMessages(prev => [...prev, loadingMessage]);

    // Simulate thinking time
    await new Promise(resolve => setTimeout(resolve, 1000));

    // Example response for creating a record
    const aiMessage = {
      id: `ai-${Date.now()}`,
      type: 'ai' as const,
      content: '',
      sql: `INSERT INTO orders (
  customer_id,
  product_id,
  quantity,
  price,
  status,
  created_at
) VALUES (
  1001,  -- Example customer ID
  2034,  -- Example product ID
  2,     -- Quantity
  29.99, -- Price per unit
  'pending',
  CURRENT_TIMESTAMP
) RETURNING *;`,
      executionTime: 78,
      result: [{
        id: 12458,
        customer_id: 1001,
        product_id: 2034,
        quantity: 2,
        price: 29.99,
        status: 'pending',
        created_at: new Date().toISOString(),
        total_amount: 59.98
      }]
    };

    // Remove loading message
    setMessages(prev => prev.filter(msg => !('isLoading' in msg)));

    // Add the AI message with empty content
    setMessages(prev => [...prev, aiMessage]);

    // Simulate typing the response
    const fullContent = "I'll help you create that record. Here's the SQL query:";
    let currentContent = '';

    for (let i = 0; i < fullContent.length; i++) {
      await new Promise(resolve => setTimeout(resolve, 30 + Math.random() * 30));
      currentContent += fullContent[i];
      setMessages(prev => prev.map(msg =>
        msg.id === aiMessage.id
          ? { ...msg, content: currentContent }
          : msg
      ));
    }

    // After typing is complete, show the SQL and result
    setMessages(prev => prev.map(msg =>
      msg.id === aiMessage.id
        ? {
          ...msg,
          content: fullContent
        }
        : msg
    ));
  };

  if (!isAuthenticated) {
    return <AuthForm onLogin={handleLogin} onSignup={handleSignup} />;
  }

  const selectedConnection = mockConnections.find(
    (conn) => conn.id === selectedConnectionId
  );

  return (
    <div className="flex flex-col md:flex-row bg-[#FFDB58]/10 min-h-screen">
      <div className="fixed top-0 left-0 right-0 h-16 bg-white border-b-4 border-black md:hidden z-50 flex items-center justify-center">
        <div className="flex items-center gap-2">
          <Boxes className="w-8 h-8" />
          <h1 className="text-2xl font-bold">NeoBase</h1>
        </div>
      </div>
      <Sidebar
        isExpanded={isSidebarExpanded}
        onToggleExpand={() => setIsSidebarExpanded(!isSidebarExpanded)}
        connections={connections}
        onSelectConnection={setSelectedConnectionId}
        onAddConnection={handleAddConnection}
        onLogout={handleLogout}
        selectedConnectionId={selectedConnectionId}
        onDeleteConnection={handleDeleteConnection}
      />

      {selectedConnection ? (
        <ChatWindow
          connectionName={selectedConnection.name}
          connectionType={selectedConnection.type}
          isExpanded={isSidebarExpanded}
          messages={messages}
          setMessages={setMessages}
          onSendMessage={(message) => {
            const newMessage = {
              id: Date.now().toString(),
              type: 'user' as const,
              content: message,
            };
            setMessages(prev => [...prev, newMessage]);
            generateAIResponse(message);
          }}
          onClearChat={handleClearChat}
          onCloseConnection={handleCloseConnection}
        />
      ) : (
        <div className={`flex-1 flex items-center justify-center p-4 mt-16 md:mt-0 min-h-[calc(100vh-4rem)] transition-all duration-300 ${isSidebarExpanded ? 'md:ml-80' : 'md:ml-20'
          }`}>
          <div className="text-center max-w-lg mx-auto">
            <h2 className="text-2xl md:text-3xl font-bold mb-6">
              Ready to explore your data?
            </h2>
            <p className="text-gray-600 text-base md:text-lg">
              <button
                onClick={handleAddConnection}
                className="text-purple-600 hover:text-purple-800 font-medium"
              >
                Create a new connection
              </button>
              {' '}or select an existing one from the sidebar to begin.
            </p>
          </div>
        </div>
      )}

      {showConnectionModal && (
        <ConnectionModal
          onClose={() => setShowConnectionModal(false)}
          onSubmit={(data) => {
            const newConnection = {
              id: Date.now().toString(),
              name: data.database,
              type: data.type,
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