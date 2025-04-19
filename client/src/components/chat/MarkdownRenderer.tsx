import React from 'react';
import ReactMarkdown from 'react-markdown';
import 'highlight.js/styles/github.css';
import './MarkdownRenderer.css';

interface MarkdownRendererProps {
  markdown: string;
  className?: string;

}

const MarkdownRenderer: React.FC<MarkdownRendererProps> = ({ markdown, className = '' }) => {
  // Apply a safeguard to ensure markdown is always a string
  const safeMarkdown = typeof markdown === 'string' ? markdown : '';
  
  return (
    <div className={`markdown-renderer ${className}`}>
      <ReactMarkdown
        skipHtml={false}
        components={{
          // Override code blocks to enhance styling
          code({ node, className, children, ...props }: any) {
            const match = /language-(\w+)/.exec(className || '');
            const isInline = !match;
            return !isInline ? (
              <div className="code-block-wrapper neo-border">
                <div className="code-language-indicator">{match ? match[1] : 'code'}</div>
                <code
                  className={className}
                  {...props}
                >
                  {children}
                </code>
              </div>
            ) : (
              <code className={className} {...props}>
                {children}
              </code>
            );
          },
          // Customize other components as needed
          a({ node, children, ...props }: any) {
            return (
              <a 
                {...props} 
                target="_blank" 
                rel="noopener noreferrer"
              >
                {children}
              </a>
            );
          },
          table({ node, children, ...props }: any) {
            return (
              <div className="overflow-x-auto">
                <table className="neo-border" {...props}>
                  {children}
                </table>
              </div>
            );
          },
          img({ node, ...props }: any) {
            return (
              <img 
                {...props} 
                className="neo-border my-2" 
                alt={props.alt || 'Image'} 
              />
            );
          },
          blockquote({ node, children, ...props }: any) {
            return (
              <blockquote className="neo-border" {...props}>
                {children}
              </blockquote>
            );
          },
          pre({ node, children, ...props }: any) {
            return (
              <pre className="neo-border" {...props}>
                {children}
              </pre>
            );
          },
          ul({ node, children, ...props }: any) {
            return (
              <ul className="list-disc ml-6" {...props}>
                {children}
              </ul>
            );
          },
          ol({ node, children, ...props }: any) {
            return (
              <ol className="list-decimal ml-6" {...props}>
                {children}
              </ol>
            );
          }
        }}
      >
        {safeMarkdown}
      </ReactMarkdown>

    </div>
  );
};

export default MarkdownRenderer; 