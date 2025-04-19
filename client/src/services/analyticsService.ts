import { initializeApp } from 'firebase/app';
import { Analytics, getAnalytics, logEvent, setUserId, setUserProperties } from 'firebase/analytics';

// Firebase configuration from environment variables
const firebaseConfig = {
  apiKey: import.meta.env.VITE_FIREBASE_API_KEY,
  authDomain: import.meta.env.VITE_FIREBASE_AUTH_DOMAIN,
  projectId: import.meta.env.VITE_FIREBASE_PROJECT_ID,
  storageBucket: import.meta.env.VITE_FIREBASE_STORAGE_BUCKET,
  messagingSenderId: import.meta.env.VITE_FIREBASE_MESSAGING_SENDER_ID,
  appId: import.meta.env.VITE_FIREBASE_APP_ID,
  measurementId: import.meta.env.VITE_FIREBASE_MEASUREMENT_ID
};

// Microsoft Clarity configuration
const clarityConfig = {
  projectId: import.meta.env.VITE_CLARITY_PROJECT_ID,
};

// Initialize Firebase
let firebaseApp;
let analytics: Analytics | undefined;

// Initialize analytics services
export const initAnalytics = () => {
  try {
    // Initialize Firebase
    firebaseApp = initializeApp(firebaseConfig);
    analytics = getAnalytics(firebaseApp);
    
    // Initialize Microsoft Clarity - using the correct method
    if (typeof window !== 'undefined' && clarityConfig.projectId) {
      // Load Clarity script programmatically
      const script = document.createElement('script');
      script.type = 'text/javascript';
      script.async = true;
      script.src = `https://www.clarity.ms/tag/${clarityConfig.projectId}`;
      
      // Add the script to the document
      const firstScript = document.getElementsByTagName('script')[0];
      if (firstScript && firstScript.parentNode) {
        firstScript.parentNode.insertBefore(script, firstScript);
      } else {
        document.head.appendChild(script);
      }
    }
    
    console.log('Analytics services initialized successfully');
  } catch (error) {
    console.error('Error initializing analytics:', error);
  }
};

// Set user identity in analytics platforms
export const identifyUser = (userId: string, username: string, createdAt: string) => {
  try {
    if (!analytics) return;
    
    // Set user ID in Firebase
    setUserId(analytics, userId);
    
    // Set user properties in Firebase
    setUserProperties(analytics, {
      username,
      created_at: createdAt,
    });
    
    // Set user in Microsoft Clarity using the window object
    if (typeof window !== 'undefined' && window.clarity) {
      window.clarity('identify', userId, {
        username,
        created_at: createdAt,
      });
    }
    
    // Log user login event
    logEvent(analytics, 'user_identified', {
      user_id: userId,
      username
    });
  } catch (error) {
    console.error('Error identifying user in analytics:', error);
  }
};

// Add a TypeScript interface for the global Window object to include clarity
declare global {
  interface Window {
    clarity: (command: string, ...args: any[]) => void;
  }
}

// Log events to Firebase Analytics
export const trackEvent = (eventName: string, eventParams = {}) => {
  try {
    if (!analytics) return;
    
    // Log event to Firebase Analytics
    logEvent(analytics, eventName, eventParams);
    
    // Also track event in Clarity if available
    if (typeof window !== 'undefined' && window.clarity) {
      window.clarity('event', eventName, eventParams);
    }
  } catch (error) {
    console.error(`Error tracking event ${eventName}:`, error);
  }
};

// User authentication events
export const trackLogin = (userId: string, username: string) => {
  trackEvent('login', { userId, username });
};

export const trackSignup = (userId: string, username: string) => {
  trackEvent('sign_up', { userId, username });
};

export const trackLogout = (userId: string, username: string) => {
  trackEvent('logout', { userId, username });
};

// Connection events
export const trackConnectionCreated = (connectionId: string, connectionType: string, connectionName: string) => {
  trackEvent('connection_created', { connectionId, connectionType, connectionName });
};

export const trackConnectionDeleted = (connectionId: string, connectionType: string, connectionName: string) => {
  trackEvent('connection_deleted', { connectionId, connectionType, connectionName });
};

export const trackConnectionEdited = (connectionId: string, connectionType: string, connectionName: string) => {
  trackEvent('connection_edited', { connectionId, connectionType, connectionName });
};

export const trackConnectionSelected = (connectionId: string, connectionType: string, connectionName: string) => {
  trackEvent('connection_selected', { connectionId, connectionType, connectionName });
};

export const trackConnectionStatusChange = (connectionId: string, isConnected: boolean) => {
  trackEvent('connection_status_change', { connectionId, isConnected });
};

// Message events
export const trackMessageSent = (chatId: string, messageLength: number) => {
  trackEvent('message_sent', { chatId, messageLength });
};

export const trackMessageEdited = (chatId: string, messageId: string) => {
  trackEvent('message_edited', { chatId, messageId });
};

export const trackChatCleared = (chatId: string) => {
  trackEvent('chat_cleared', { chatId });
};

// Schema events
export const trackSchemaRefreshed = (connectionId: string, connectionName: string) => {
  trackEvent('schema_refreshed', { connectionId, connectionName });
};

export const trackSchemaCancelled = (connectionId: string, connectionName: string) => {
  trackEvent('schema_refresh_cancelled', { connectionId, connectionName });
};

// Query events
export const trackQueryExecuted = (chatId: string, queryLength: number, success: boolean) => {
  trackEvent('query_executed', { chatId, queryLength, success });
};

export const trackQueryCancelled = (chatId: string) => {
  trackEvent('query_cancelled', { chatId });
};

// UI events
export const trackSidebarToggled = (isExpanded: boolean) => {
  trackEvent('sidebar_toggled', { isExpanded });
};

// Add this function with the other tracking functions
export const trackConnectionDuplicated = (connectionId: string, connectionType: string, databaseName: string, withMessages: boolean) => {
  try {
    trackEvent('connection_duplicated', {
      connectionId,
      connectionType,
      databaseName,
      withMessages
    });
    console.log('[Analytics] Tracked Connection Duplicated event');
  } catch (error) {
    console.error('[Analytics] Failed to track Connection Duplicated event:', error);
  }
};

// Export default service
const analyticsService = {
  initAnalytics,
  identifyUser,
  trackEvent,
  trackLogin,
  trackSignup,
  trackLogout,
  trackConnectionCreated,
  trackConnectionDeleted,
  trackConnectionEdited,
  trackConnectionSelected,
  trackConnectionStatusChange,
  trackMessageSent,
  trackMessageEdited,
  trackChatCleared,
  trackSchemaRefreshed,
  trackSchemaCancelled,
  trackQueryExecuted,
  trackQueryCancelled,
  trackSidebarToggled,
  trackConnectionDuplicated
};

export default analyticsService; 