import { Send } from 'lucide-react';
import { FormEvent, useState } from 'react';

interface MessageInputProps {
    isConnected: boolean;
    onSendMessage: (message: string) => void;
    isExpanded: boolean;
}

export default function MessageInput({ isConnected, onSendMessage, isExpanded }: MessageInputProps) {
    const [input, setInput] = useState('');

    const handleSubmit = (e: FormEvent) => {
        e.preventDefault();
        if (input.trim()) {
            onSendMessage(input.trim());
            setInput('');
        }
    };

    return (
        <form
            onSubmit={handleSubmit}
            className={`
            fixed bottom-0 left-0 right-0 p-4 
            bg-white border-t-4 border-black 
            transition-all duration-300
            z-[10]
            ${isExpanded
                    ? `
                    [@media(min-width:1024px)_and_(max-width:1279px)]:ml-[20rem]
                    [@media(min-width:1280px)_and_(max-width:1439px)]:ml-[20rem]
                    [@media(min-width:1440px)_and_(max-width:1700px)]:ml-[18rem]
                    ml-2
                `
                    : 'md:left-[5rem]'
                }
        `}
        >
            <div className="max-w-5xl mx-auto flex gap-4 chat-input-1440">
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
                    <Send className="w-6 h-8" />
                </button>
            </div>
        </form>
    );
}