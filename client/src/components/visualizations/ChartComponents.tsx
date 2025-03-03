import React from 'react';
import {
  BarChart,
  Bar,
  LineChart,
  Line,
  PieChart,
  Pie,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  Legend,
  ResponsiveContainer,
  Cell
} from 'recharts';
import { VisualizationData } from '../../types/visualization';

// Array of colors for charts
const COLORS = [
  '#8884d8', '#83a6ed', '#8dd1e1', '#82ca9d', '#a4de6c',
  '#d0ed57', '#ffc658', '#ff8042', '#ff6361', '#bc5090'
];

interface ChartProps {
  data: VisualizationData;
}

export const BarChartComponent: React.FC<ChartProps> = ({ data }) => {
  // Format data for recharts
  const formattedData = Object.entries(data.data).map(([key, value]) => {
    // Flatten any array data into individual records
    if (Array.isArray(value)) {
      // Handle array data with objects
      return value.map((item: any) => {
        if (typeof item === 'object') {
          return { ...item };
        }
        // Handle array of primitive values
        return { name: key, value: item };
      });
    }
    
    // Handle non-array data
    if (typeof value === 'object') {
      return Object.entries(value).map(([k, v]) => ({ name: k, value: v }));
    }
    
    return { name: key, value };
  }).flat();

  // If data is empty, show placeholder
  if (formattedData.length === 0) {
    return <div className="p-4 text-center text-gray-500">No data available</div>;
  }

  return (
    <div className="w-full h-96 p-4">
      <h3 className="text-xl font-bold mb-2">{data.title}</h3>
      <p className="text-sm text-gray-600 mb-4">{data.description}</p>
      <ResponsiveContainer width="100%" height={300}>
        <BarChart data={formattedData}>
          <CartesianGrid strokeDasharray="3 3" />
          <XAxis dataKey="name" />
          <YAxis />
          <Tooltip />
          <Legend />
          <Bar dataKey="value" fill="#8884d8" />
        </BarChart>
      </ResponsiveContainer>
    </div>
  );
};

export const LineChartComponent: React.FC<ChartProps> = ({ data }) => {
  // Format data for recharts (similar to bar chart but optimized for time series)
  const formattedData = Object.entries(data.data).map(([key, value]) => {
    if (Array.isArray(value)) {
      return value.map((item: any) => {
        if (typeof item === 'object') {
          return { ...item };
        }
        return { name: key, value: item };
      });
    }
    
    if (typeof value === 'object') {
      return Object.entries(value).map(([k, v]) => ({ name: k, value: v }));
    }
    
    return { name: key, value };
  }).flat();

  if (formattedData.length === 0) {
    return <div className="p-4 text-center text-gray-500">No data available</div>;
  }

  return (
    <div className="w-full h-96 p-4">
      <h3 className="text-xl font-bold mb-2">{data.title}</h3>
      <p className="text-sm text-gray-600 mb-4">{data.description}</p>
      <ResponsiveContainer width="100%" height={300}>
        <LineChart data={formattedData}>
          <CartesianGrid strokeDasharray="3 3" />
          <XAxis dataKey="name" />
          <YAxis />
          <Tooltip />
          <Legend />
          <Line type="monotone" dataKey="value" stroke="#8884d8" activeDot={{ r: 8 }} />
        </LineChart>
      </ResponsiveContainer>
    </div>
  );
};

export const PieChartComponent: React.FC<ChartProps> = ({ data }) => {
  // Format data for recharts
  const formattedData = Object.entries(data.data).map(([key, value]) => {
    if (Array.isArray(value)) {
      return value.map((item: any) => {
        if (typeof item === 'object') {
          return { ...item };
        }
        return { name: key, value: item };
      });
    }
    
    if (typeof value === 'object') {
      return Object.entries(value).map(([k, v]) => ({ name: k, value: v }));
    }
    
    return { name: key, value };
  }).flat();

  if (formattedData.length === 0) {
    return <div className="p-4 text-center text-gray-500">No data available</div>;
  }

  return (
    <div className="w-full h-96 p-4">
      <h3 className="text-xl font-bold mb-2">{data.title}</h3>
      <p className="text-sm text-gray-600 mb-4">{data.description}</p>
      <ResponsiveContainer width="100%" height={300}>
        <PieChart>
          <Pie
            data={formattedData}
            cx="50%"
            cy="50%"
            labelLine={false}
            label={({ name, percent }) => `${name}: ${(percent * 100).toFixed(0)}%`}
            outerRadius={80}
            fill="#8884d8"
            dataKey="value"
          >
            {formattedData.map((entry, index) => (
              <Cell key={`cell-${index}`} fill={COLORS[index % COLORS.length]} />
            ))}
          </Pie>
          <Tooltip />
          <Legend />
        </PieChart>
      </ResponsiveContainer>
    </div>
  );
};

export const TimeSeriesChartComponent: React.FC<ChartProps> = ({ data }) => {
  // Format data for recharts optimized for time series
  const formattedData = Object.entries(data.data).map(([key, value]) => {
    if (Array.isArray(value)) {
      return value.map((item: any) => {
        if (typeof item === 'object') {
          return { ...item };
        }
        return { time: key, value: item };
      });
    }
    
    if (typeof value === 'object') {
      return Object.entries(value).map(([k, v]) => ({ time: k, value: v }));
    }
    
    return { time: key, value };
  }).flat();

  if (formattedData.length === 0) {
    return <div className="p-4 text-center text-gray-500">No data available</div>;
  }

  return (
    <div className="w-full h-96 p-4">
      <h3 className="text-xl font-bold mb-2">{data.title}</h3>
      <p className="text-sm text-gray-600 mb-4">{data.description}</p>
      <ResponsiveContainer width="100%" height={300}>
        <LineChart data={formattedData}>
          <CartesianGrid strokeDasharray="3 3" />
          <XAxis dataKey="time" />
          <YAxis />
          <Tooltip />
          <Legend />
          <Line type="monotone" dataKey="value" stroke="#8884d8" activeDot={{ r: 8 }} />
        </LineChart>
      </ResponsiveContainer>
    </div>
  );
};

export const TableComponent: React.FC<ChartProps> = ({ data }) => {
  // Format data for a table
  const formattedData = Array.isArray(data.data) 
    ? data.data 
    : Object.entries(data.data).map(([key, value]) => {
        if (Array.isArray(value)) {
          return value;
        }
        return { [key]: value };
      }).flat();

  if (formattedData.length === 0) {
    return <div className="p-4 text-center text-gray-500">No data available</div>;
  }

  // Extract column headers
  const firstRow = formattedData[0];
  const headers = Object.keys(firstRow || {});

  return (
    <div className="w-full p-4 overflow-x-auto">
      <h3 className="text-xl font-bold mb-2">{data.title}</h3>
      <p className="text-sm text-gray-600 mb-4">{data.description}</p>
      <table className="min-w-full divide-y divide-gray-200">
        <thead className="bg-gray-50">
          <tr>
            {headers.map((header, idx) => (
              <th 
                key={idx}
                scope="col" 
                className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider"
              >
                {header}
              </th>
            ))}
          </tr>
        </thead>
        <tbody className="bg-white divide-y divide-gray-200">
          {formattedData.map((row, rowIdx) => (
            <tr key={rowIdx}>
              {headers.map((header, colIdx) => (
                <td key={colIdx} className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                  {JSON.stringify(row[header])}
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}; 