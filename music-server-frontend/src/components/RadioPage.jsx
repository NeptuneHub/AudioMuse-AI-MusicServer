import React, { useState } from 'react';
import Radio from './Radio.jsx';
import SongAlchemy from './SongAlchemy.jsx';

export default function RadioPage({ onNavigate, onAddToQueue, onPlay }) {
  const [activeTab, setActiveTab] = useState('stations'); // 'stations' or 'create'

  return (
    <div>
      {/* Tab Navigation */}
      <div className="mb-6 border-b border-gray-700">
        <div className="flex gap-4">
          <button
            onClick={() => setActiveTab('stations')}
            className={`px-6 py-3 font-semibold transition-all relative ${
              activeTab === 'stations'
                ? 'text-teal-400 border-b-2 border-teal-400'
                : 'text-gray-400 hover:text-white'
            }`}
          >
            <div className="flex items-center gap-2">
              <svg className="w-5 h-5" fill="currentColor" viewBox="0 0 20 20">
                <path d="M2 6a2 2 0 012-2h6a2 2 0 012 2v8a2 2 0 01-2 2H4a2 2 0 01-2-2V6zM14.553 7.106A1 1 0 0014 8v4a1 1 0 00.553.894l2 1A1 1 0 0018 13V7a1 1 0 00-1.447-.894l-2 1z"></path>
              </svg>
              My Stations
            </div>
          </button>
          <button
            onClick={() => setActiveTab('create')}
            className={`px-6 py-3 font-semibold transition-all relative ${
              activeTab === 'create'
                ? 'text-teal-400 border-b-2 border-teal-400'
                : 'text-gray-400 hover:text-white'
            }`}
          >
            <div className="flex items-center gap-2">
              <svg className="w-5 h-5" fill="currentColor" viewBox="0 0 20 20">
                <path fillRule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm1-11a1 1 0 10-2 0v2H7a1 1 0 100 2h2v2a1 1 0 102 0v-2h2a1 1 0 100-2h-2V7z" clipRule="evenodd"></path>
              </svg>
              Create Station
            </div>
          </button>
        </div>
      </div>

      {/* Tab Content */}
      <div>
        {activeTab === 'stations' && (
          <Radio 
            onNavigate={onNavigate} 
            onAddToQueue={onAddToQueue} 
            onPlay={onPlay}
            onSwitchToCreate={() => setActiveTab('create')}
          />
        )}
        {activeTab === 'create' && (
          <SongAlchemy 
            onNavigate={onNavigate} 
            onAddToQueue={onAddToQueue} 
            onPlay={onPlay}
            onRadioCreated={() => setActiveTab('stations')}
          />
        )}
      </div>
    </div>
  );
}
