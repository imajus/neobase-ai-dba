import { createContext, ReactNode, useContext, useEffect, useState } from 'react';
import { v4 as uuidv4 } from 'uuid';

interface StreamContextType {
    streamId: string | null; // 32 bytes strictly
    setStreamId: (id: string | null) => void;
    generateStreamId: () => string; // Add function to generate stream ID
}

const StreamContext = createContext<StreamContextType | undefined>(undefined);

export function StreamProvider({ children }: { children: ReactNode }) {
    const [streamId, setStreamId] = useState<string | null>(null);

    // Generate a 32-byte stream ID
    const generateStreamId = () => {
        // UUID v4 generates a 32-byte hex string
        return uuidv4().replace(/-/g, '');
    };

    // Initialize streamId if not set
    useEffect(() => {
        if (!streamId) {
            setStreamId(generateStreamId());
        }
    }, []);

    return (
        <StreamContext.Provider value={{ streamId, setStreamId, generateStreamId }}>
            {children}
        </StreamContext.Provider>
    );
}

export function useStream() {
    const context = useContext(StreamContext);
    if (context === undefined) {
        throw new Error('useStream must be used within a StreamProvider');
    }
    return context;
} 