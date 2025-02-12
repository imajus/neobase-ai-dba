import { Message } from "../components/chat/types";

const mockMessages: Message[] = [
    {
        id: 'user-1',
        type: 'user',
        content: 'Show me all active users in the database',
    },
    {
        id: 'ai-1',
        type: 'ai',
        content: 'Here are all users in the database:',
        queries: [{
            id: 'q1',
            canRollback: false,
            query: 'SELECT * FROM users WHERE active = true ORDER BY last_login DESC LIMIT 10;',
            executionTime: 42,
            exampleResult: [
                { id: 1, email: 'john@example.com', last_login: '2024-03-10T15:30:00Z', active: true },
                { id: 2, email: 'sarah@example.com', last_login: '2024-03-10T14:45:00Z', active: true },
                { id: 3, email: 'mike@example.com', last_login: '2024-03-10T13:20:00Z', active: true }
            ],
            executionResult: [
                { id: 1, email: 'john@example.com', last_login: '2024-03-10T15:30:00Z', active: true },
                { id: 2, email: 'sarah@example.com', last_login: '2024-03-10T14:45:00Z', active: true },
                { id: 3, email: 'mike@example.com', last_login: '2024-03-10T13:20:00Z', active: true }
            ]
        }]
    },
    {
        id: 'user-2',
        type: 'user',
        content: 'How many orders were placed in the last 24 hours?',
    },
    {
        id: 'ai-2',
        type: 'ai',
        content: 'I\'ll check the orders from the last 24 hours:',
        queries: [{
            id: 'q2',
            canRollback: false,
            query: 'SELECT COUNT(*) as order_count FROM orders WHERE created_at >= NOW() - INTERVAL \'24 hours\';',
            executionTime: 156,
            exampleResult: [
                { order_count: 157 }
            ],
            executionResult: [
                { order_count: 157 }
            ]
        }]
    },
    {
        id: 'user-3',
        type: 'user',
        content: 'What are our top 5 selling products this month?',
    },
    {
        id: 'ai-3',
        type: 'ai',
        content: 'Here are the top 5 selling products for this month:',
        queries: [{
            id: 'q3',
            canRollback: false,
            query: `SELECT
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
            exampleResult: [
                { name: 'Wireless Earbuds Pro', total_sold: 245, revenue: 24500.00 },
                { name: 'Smart Watch X3', total_sold: 189, revenue: 37800.00 },
                { name: 'Gaming Mouse', total_sold: 156, revenue: 7800.00 },
                { name: 'Mechanical Keyboard', total_sold: 134, revenue: 13400.00 },
                { name: 'USB-C Hub', total_sold: 98, revenue: 2940.00 }
            ],
            executionResult: [
                { name: 'Wireless Earbuds Pro', total_sold: 245, revenue: 24500.00 },
                { name: 'Smart Watch X3', total_sold: 189, revenue: 37800.00 },
                { name: 'Gaming Mouse', total_sold: 156, revenue: 7800.00 },
                { name: 'Mechanical Keyboard', total_sold: 134, revenue: 13400.00 },
                { name: 'USB-C Hub', total_sold: 98, revenue: 2940.00 }
            ]
        }]
    },
    {
        id: 'user-4',
        type: 'user',
        content: 'Show me all transactions with invalid amounts',
    },
    {
        id: 'ai-4',
        type: 'ai',
        content: 'Here are the transactions with invalid amounts:',
        queries: [{
            id: 'q4',
            canRollback: false,
            query: `SELECT
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
        }]
    },
    {
        id: 'user-5',
        type: 'user',
        content: 'Delete all inactive users who haven\'t logged in for the past 6 months',
    },
    {
        id: 'ai-5',
        type: 'ai',
        content: 'Got it! Here\'s the query to delete inactive users:',
        queries: [{
            id: 'q5',
            canRollback: true,
            query: `DELETE FROM users 
WHERE active = false 
AND last_login < NOW() - INTERVAL '6 months'
RETURNING id, email, last_login;`,
            executionTime: 189,
            executionResult: [
                { id: 45, email: 'old_user1@example.com', last_login: '2023-08-15T10:20:00Z' },
                { id: 67, email: 'inactive_user@example.com', last_login: '2023-09-01T08:45:00Z' },
                { id: 89, email: 'dormant@example.com', last_login: '2023-07-22T14:30:00Z' }
            ],
            exampleResult: [
                { id: 45, email: 'old_user1@example.com', last_login: '2023-08-15T10:20:00Z' },
                { id: 67, email: 'inactive_user@example.com', last_login: '2023-09-01T08:45:00Z' },
                { id: 89, email: 'dormant@example.com', last_login: '2023-07-22T14:30:00Z' }
            ],
            isCritical: true
        }]
    }
];

export const newMockMessage: Message = {
    id: 'ai-mock-1',
    type: 'ai',
    content: 'Here are the commands to create a new order, and then get orders with a status of "pending". First execute the insert command, then the select command:',
    queries: [{
        id: 'q6',
        canRollback: true,
        executionTime: 123,
        exampleResult: [{
            id: 12458,
            customer_id: 1001,
            product_id: 2034,
            quantity: 2,
            price: 29.99,
            status: 'pending',
            created_at: new Date().toISOString(),
            total_amount: 59.98
        }],
        executionResult: [{
            id: 12458,
            customer_id: 1001,
            product_id: 2034,
            quantity: 2,
            price: 29.99,
            status: 'pending',
            created_at: new Date().toISOString(),
            total_amount: 59.98
        }],
        query: `INSERT INTO orders (
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
    }, {
        id: 'q7',
        canRollback: false,
        query: `SELECT * FROM orders WHERE status = 'pending';`,
        executionTime: 78,
        exampleResult: [
            {
                id: 12458,
                customer_id: 1001,
                product_id: 2034,
                quantity: 2,
                price: 29.99,
                status: 'pending',
                created_at: new Date().toISOString(),
                total_amount: 59.98
            }
        ],
        executionResult: [
            {
                id: 12458,
                customer_id: 1001,
                product_id: 2034,
                quantity: 2,
                price: 29.99,
                status: 'pending',
                created_at: new Date().toISOString(),
                total_amount: 59.98
            }
        ]
    }]
};

export default mockMessages;