import React, { useState, useEffect } from 'react';
import { getRadios, getRadioSeed, deleteRadio, updateRadioName } from '../api';

export default function Radio({ onNavigate, onAddToQueue, onPlay, onSwitchToCreate }) {
  const [radios, setRadios] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [editingId, setEditingId] = useState(null);
  const [editingName, setEditingName] = useState('');
  const [playingRadio, setPlayingRadio] = useState(null);

  useEffect(() => {
    loadRadios();
  }, []);

  const loadRadios = async () => {
    setLoading(true);
    try {
      const data = await getRadios();
      setRadios(data.radios || []);
    } catch (err) {
      setError('Failed to load radio stations');
      console.error(err);
    }
    setLoading(false);
  };

  const handlePlayRadio = async (radio) => {
    setPlayingRadio(radio.id);
    setError('');

    try {
      // Get the radio seed configuration
      const seedData = await getRadioSeed(radio.id);
      
      // Parse seed songs
      const items = JSON.parse(seedData.seed_songs);

      // Run alchemy with n=200
      const alchemyPayload = {
        items,
        n: 200,
        temperature: seedData.temperature,
        subtract_distance: seedData.subtract_distance
      };

      const response = await fetch('/api/alchemy', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${localStorage.getItem('token')}`
        },
        body: JSON.stringify(alchemyPayload)
      });

      const data = await response.json();
      
      if (!response.ok || data.error) {
        setError(data.error || 'Failed to generate radio playlist');
        setPlayingRadio(null);
        return;
      }

      // Map results to song objects
      const mapped = (data.results || []).map(r => ({
        id: r.item_id || r.id || r.songId || '',
        title: r.title || r.name || '',
        artist: r.author || r.artist || r.creator || '',
        // Store radio metadata for auto-rerun
        _radioId: radio.id,
        _radioName: radio.name,
        _isRadioSong: true
      }));

      // Play the first song directly and add rest to queue
      if (mapped.length > 0 && onPlay) {
        onPlay(mapped[0], mapped);
      }

    } catch (err) {
      setError('Failed to start radio');
      console.error(err);
    }
    setPlayingRadio(null);
  };

  const handleDelete = async (radioId) => {
    if (!window.confirm('Are you sure you want to delete this radio station?')) {
      return;
    }

    try {
      await deleteRadio(radioId);
      setRadios(radios.filter(r => r.id !== radioId));
    } catch (err) {
      setError('Failed to delete radio');
      console.error(err);
    }
  };

  const handleStartEdit = (radio) => {
    setEditingId(radio.id);
    setEditingName(radio.name);
  };

  const handleSaveEdit = async (radioId) => {
    if (!editingName.trim()) {
      setError('Radio name cannot be empty');
      return;
    }

    try {
      await updateRadioName(radioId, editingName);
      setRadios(radios.map(r => r.id === radioId ? { ...r, name: editingName } : r));
      setEditingId(null);
      setEditingName('');
    } catch (err) {
      setError('Failed to update radio name');
      console.error(err);
    }
  };

  const handleCancelEdit = () => {
    setEditingId(null);
    setEditingName('');
  };

  if (loading) {
    return (
      <div className="flex justify-center items-center h-64">
        <svg className="animate-spin h-12 w-12 text-teal-400" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
          <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"></circle>
          <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
        </svg>
      </div>
    );
  }

  return (
    <div className="text-gray-100">
      {/* Header */}
      <div className="mb-6">
        <h1 className="text-3xl font-bold text-white flex items-center gap-3">
          <svg className="w-8 h-8 text-teal-400" fill="currentColor" viewBox="0 0 20 20">
            <path d="M2 6a2 2 0 012-2h6a2 2 0 012 2v8a2 2 0 01-2 2H4a2 2 0 01-2-2V6zM14.553 7.106A1 1 0 0014 8v4a1 1 0 00.553.894l2 1A1 1 0 0018 13V7a1 1 0 00-1.447-.894l-2 1z"></path>
          </svg>
          Radio Stations
        </h1>
        <p className="text-gray-400 mt-2">
          Your personalized AI-powered radio stations • {radios.length} {radios.length === 1 ? 'station' : 'stations'}
        </p>
      </div>

      {error && (
        <div className="mb-6 p-4 bg-red-500/10 border border-red-500 rounded-lg text-red-400">
          {error}
        </div>
      )}

      {/* Radio Grid */}
      {radios.length === 0 ? (
        <div className="text-center py-16">
          <svg className="w-24 h-24 text-gray-600 mx-auto mb-4" fill="currentColor" viewBox="0 0 20 20">
            <path d="M2 6a2 2 0 012-2h6a2 2 0 012 2v8a2 2 0 01-2 2H4a2 2 0 01-2-2V6zM14.553 7.106A1 1 0 0014 8v4a1 1 0 00.553.894l2 1A1 1 0 0018 13V7a1 1 0 00-1.447-.894l-2 1z"></path>
          </svg>
          <h3 className="text-xl font-semibold text-gray-400 mb-2">No Radio Stations Yet</h3>
          <p className="text-gray-500 mb-6">Create your first radio station using Song Alchemy</p>
          <button
            onClick={() => onSwitchToCreate && onSwitchToCreate()}
            className="border-2 border-teal-500 text-teal-400 bg-teal-500/10 hover:bg-teal-500/20 hover:scale-105 transition-all px-6 py-3 rounded-lg font-semibold"
          >
            Create Your First Station
          </button>
        </div>
      ) : (
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4">
          {radios.map((radio) => (
            <div
              key={radio.id}
              className="bg-gradient-to-br from-gray-800 to-gray-900 rounded-xl p-6 border border-gray-700 hover:border-teal-500/50 transition-all shadow-lg hover:shadow-teal-500/20 group"
            >
              {/* Radio Icon */}
              <div className="mb-4 flex justify-center">
                <div className="w-20 h-20 bg-gradient-to-br from-teal-500/20 to-purple-500/20 rounded-full flex items-center justify-center group-hover:scale-110 transition-transform">
                  <svg className="w-10 h-10 text-teal-400" fill="currentColor" viewBox="0 0 20 20">
                    <path d="M2 6a2 2 0 012-2h6a2 2 0 012 2v8a2 2 0 01-2 2H4a2 2 0 01-2-2V6zM14.553 7.106A1 1 0 0014 8v4a1 1 0 00.553.894l2 1A1 1 0 0018 13V7a1 1 0 00-1.447-.894l-2 1z"></path>
                  </svg>
                </div>
              </div>

              {/* Radio Name */}
              {editingId === radio.id ? (
                <div className="mb-4">
                  <input
                    type="text"
                    value={editingName}
                    onChange={(e) => setEditingName(e.target.value)}
                    className="w-full bg-gray-900 border border-teal-500 rounded px-3 py-2 text-white focus:outline-none focus:ring-2 focus:ring-teal-500/50"
                    autoFocus
                  />
                  <div className="flex gap-2 mt-2">
                    <button
                      onClick={() => handleSaveEdit(radio.id)}
                      className="flex-1 bg-teal-500/20 hover:bg-teal-500/30 text-teal-400 px-3 py-1 rounded text-sm"
                    >
                      Save
                    </button>
                    <button
                      onClick={handleCancelEdit}
                      className="flex-1 bg-gray-700 hover:bg-gray-600 text-gray-300 px-3 py-1 rounded text-sm"
                    >
                      Cancel
                    </button>
                  </div>
                </div>
              ) : (
                <h3 className="text-lg font-semibold text-white mb-2 text-center truncate">
                  {radio.name}
                </h3>
              )}

              {/* Metadata */}
              <div className="text-xs text-gray-400 text-center mb-4 space-y-1">
                <div>Temperature: {radio.temperature.toFixed(1)}</div>
                <div className="text-gray-500">
                  {new Date(radio.created_at).toLocaleDateString()}
                </div>
              </div>

              {/* Action Buttons */}
              <div className="space-y-2">
                <button
                  onClick={() => handlePlayRadio(radio)}
                  disabled={playingRadio === radio.id}
                  className="w-full bg-gradient-to-r from-teal-500 to-purple-500 hover:from-teal-400 hover:to-purple-400 text-white font-semibold py-3 px-4 rounded-lg transition-all hover:scale-105 disabled:opacity-50 disabled:cursor-not-allowed disabled:hover:scale-100 flex items-center justify-center gap-2"
                >
                  {playingRadio === radio.id ? (
                    <>
                      <svg className="animate-spin h-5 w-5" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
                        <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"></circle>
                        <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                      </svg>
                      Loading...
                    </>
                  ) : (
                    <>
                      <svg className="w-5 h-5" fill="currentColor" viewBox="0 0 20 20">
                        <path d="M6.3 2.841A1.5 1.5 0 004 4.11V15.89a1.5 1.5 0 002.3 1.269l9.344-5.89a1.5 1.5 0 000-2.538L6.3 2.84z"></path>
                      </svg>
                      Play Radio
                    </>
                  )}
                </button>

                <div className="flex gap-2">
                  <button
                    onClick={() => handleStartEdit(radio)}
                    className="flex-1 bg-gray-700 hover:bg-gray-600 text-gray-300 py-2 px-3 rounded-lg transition-all flex items-center justify-center gap-1 text-sm"
                  >
                    <svg className="w-4 h-4" fill="currentColor" viewBox="0 0 20 20">
                      <path d="M13.586 3.586a2 2 0 112.828 2.828l-.793.793-2.828-2.828.793-.793zM11.379 5.793L3 14.172V17h2.828l8.38-8.379-2.83-2.828z"></path>
                    </svg>
                    Rename
                  </button>
                  <button
                    onClick={() => handleDelete(radio.id)}
                    className="flex-1 bg-red-500/20 hover:bg-red-500/30 text-red-400 py-2 px-3 rounded-lg transition-all flex items-center justify-center gap-1 text-sm"
                  >
                    <svg className="w-4 h-4" fill="currentColor" viewBox="0 0 20 20">
                      <path fillRule="evenodd" d="M9 2a1 1 0 00-.894.553L7.382 4H4a1 1 0 000 2v10a2 2 0 002 2h8a2 2 0 002-2V6a1 1 0 100-2h-3.382l-.724-1.447A1 1 0 0011 2H9zM7 8a1 1 0 012 0v6a1 1 0 11-2 0V8zm5-1a1 1 0 00-1 1v6a1 1 0 102 0V8a1 1 0 00-1-1z" clipRule="evenodd"></path>
                    </svg>
                    Delete
                  </button>
                </div>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
