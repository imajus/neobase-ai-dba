import { Eraser, Loader, Pencil, PlugZap, RefreshCw } from 'lucide-react';
import { useMemo } from 'react';
import { Chat } from '../../types/chat';
import DatabaseLogo from '../icons/DatabaseLogos';

interface ChatHeaderProps {
    chat: Chat;
    isConnecting: boolean;
    isConnected: boolean;
    onClearChat: () => void;
    onEditConnection: () => void;
    onShowCloseConfirm: () => void;
    onReconnect: () => void;
}

export default function ChatHeader({
    chat,
    isConnecting = true,
    isConnected,
    onClearChat,
    onEditConnection,
    onShowCloseConfirm,
    onReconnect,
}: ChatHeaderProps) {
    const connectionStatus = useMemo(() => {
        if (isConnecting) {
            return (
                <span className="text-yellow-600 text-sm font-medium bg-yellow-100 px-2 py-1 rounded flex items-center gap-2">
                    <Loader className="w-3 h-3 animate-spin" />
                    Connecting...
                </span>
            );
        }
        return isConnected ? (
            <span className="text-emerald-700 text-sm font-medium bg-emerald-100 px-2 py-1 rounded">
                Connected
            </span>
        ) : (
            <span className="text-neo-error text-sm font-medium bg-neo-error/10 px-2 py-1 rounded">
                Disconnected
            </span>
        );
    }, [isConnecting, isConnected]);

    return (
        <div className="fixed top-0 left-0 right-0 md:relative md:left-auto md:right-auto bg-white border-b-4 border-black h-16 px-4 flex justify-between items-center mt-16 md:mt-0 z-20">
            <div className="flex items-center gap-2 overflow-hidden max-w-[60%]">
                <DatabaseLogo type={chat.connection.type as "postgresql" | "mysql" | "mongodb" | "redis" | "clickhouse" | "neo4j"} size={32} className="transition-transform hover:scale-110" />
                <h2 className="text-lg md:text-2xl font-bold truncate">{chat.connection.database}</h2>
                {connectionStatus}
            </div>
            <div className="flex items-center gap-2">
                <button
                    onClick={onClearChat}
                    className="p-2 text-neo-error hover:bg-neo-error/10 rounded-lg transition-colors hidden md:block neo-border"
                    title="Clear chat"
                >
                    <Eraser className="w-5 h-5" />
                </button>
                <button
                    onClick={onEditConnection}
                    className="p-2 hover:bg-neo-gray rounded-lg transition-colors hidden md:block neo-border text-gray-800"
                    title="Edit connection"
                >
                    <Pencil className="w-5 h-5" />
                </button>
                {isConnected ? (
                    <button
                        onClick={onShowCloseConfirm}
                        className="p-2 hover:bg-neo-gray rounded-lg transition-colors hidden md:block neo-border text-gray-800"
                        title="Disconnect connection"
                    >
                        <PlugZap className="w-5 h-5" />
                    </button>
                ) : (
                    <button
                        onClick={onReconnect}
                        className="p-2 hover:bg-neo-gray rounded-lg transition-colors hidden md:block neo-border"
                        title="Reconnect"
                    >
                        <RefreshCw className="w-5 h-5 text-gray-800" />
                    </button>
                )}
                {/* Mobile buttons without borders */}
                <button
                    onClick={onClearChat}
                    className="p-2 text-neo-error hover:bg-neo-error/10 rounded-lg transition-colors md:hidden"
                    title="Clear chat"
                >
                    <Eraser className="w-5 h-5" />
                </button>
                <button
                    onClick={onEditConnection}
                    className="p-2 hover:bg-neo-gray rounded-lg transition-colors md:hidden"
                    title="Edit connection"
                >
                    <Pencil className="w-5 h-5" />
                </button>
                {isConnected ? (
                    <button
                        onClick={onShowCloseConfirm}
                        className="p-2 hover:bg-neo-gray rounded-lg transition-colors md:hidden"
                        title="Disconnect connection"
                    >
                        <PlugZap className="w-5 h-5 text-gray-800" />
                    </button>
                ) : (
                    <button
                        onClick={onReconnect}
                        className="p-2 hover:bg-neo-gray rounded-lg transition-colors md:hidden"
                        title="Reconnect"
                    >
                        <RefreshCw className="w-5 h-5 text-gray-800" />
                    </button>
                )}
            </div>
        </div>
    );
}