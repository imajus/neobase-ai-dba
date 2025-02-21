import { createContext, ReactNode, useCallback, useContext, useEffect, useState } from 'react';

interface StreamContextType {
    streamId: string | null; // 32 bytes strictly
    setStreamId: (id: string | null) => void;
    generateStreamId: () => string; // Add function to generate stream ID
}

const StreamContext = createContext<StreamContextType | undefined>(undefined);

const generateId = () => {
    const timestamp = Date.now().toString(36);
    const randomStr = Math.random().toString(36).substring(2, 8);
    return `${timestamp}-${randomStr}`;
};

export const StreamProvider = ({ children }: { children: ReactNode }) => {
    const [streamId, setStreamId] = useState<string | null>(null);

    const generateStreamId = useCallback(() => {
        const newStreamId = generateId();
        setStreamId(newStreamId);
        return newStreamId;
    }, []);

    // Initialize streamId if not set
    useEffect(() => {
        if (!streamId) {
            generateStreamId();
        }
    }, [generateStreamId]);

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