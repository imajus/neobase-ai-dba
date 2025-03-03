export type VisualizationType = 'bar_chart' | 'line_chart' | 'pie_chart' | 'table' | 'time_series';

export interface VisualizationSuggestion {
    title: string;
    description: string;
    visualization_type: VisualizationType;
    query: string;
    table_name: string;
}

export interface VisualizationData {
    title: string;
    description: string;
    visualization_type: VisualizationType;
    query: string;
    table_name: string;
    data: Record<string, any>;
    execution_time: number;
}

export interface TableSuggestionsResponse {
    success: boolean;
    data: VisualizationSuggestion[];
}

export interface AllSuggestionsResponse {
    success: boolean;
    data: Record<string, VisualizationSuggestion[]>;
}

export interface VisualizationDataResponse {
    success: boolean;
    data: VisualizationData;
} 