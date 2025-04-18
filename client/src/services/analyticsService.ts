import { initializeApp } from 'firebase/app';
import { Analytics, getAnalytics, logEvent, setUserId, setUserProperties } from 'firebase/analytics';
import clarity from 'react-microsoft-clarity';

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
  track: true,
  content: true,
  debug: false,
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
    
    // Initialize Microsoft Clarity
    if (typeof window !== 'undefined' && clarityConfig.projectId) {
      // @ts-ignore - Clarity types are not properly defined
      clarity.init(clarityConfig.projectId);
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
    
    // Set user in Microsoft Clarity
    // @ts-ignore - Clarity types are not properly defined
    clarity.identify(userId, {
      username,
      created_at: createdAt,
    });
    
    // Log user login event
    logEvent(analytics, 'user_identified', {
      user_id: userId,
      username
    });
  } catch (error) {
    console.error('Error identifying user in analytics:', error);
  }
};

// Log events to Firebase Analytics
export const trackEvent = (eventName: string, eventParams = {}) => {
  try {
    if (!analytics) return;
    
    // Log event to Firebase Analytics
    logEvent(analytics, eventName, eventParams);
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
  trackSidebarToggled
};

export default analyticsService; 