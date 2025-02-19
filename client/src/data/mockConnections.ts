import { Chat } from "../components/modals/ConnectionModal";

// Mock data for demonstration
const mockConnections: Chat[] = [
    { id: '1', type: 'postgresql' as const, host: 'localhost', port: '5432', username: 'postgres', password: 'postgres', database: 'Nps-uat' },
    { id: '2', type: 'mysql' as const, host: 'localhost', port: '3306', username: 'root', password: 'root', database: 'Jobprot-dev' },
];

export default mockConnections;