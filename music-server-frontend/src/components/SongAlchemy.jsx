import React, { useState, useRef, useEffect } from 'react';
import { searchMusic, createRadio } from '../api';
import Plotly from 'plotly.js-dist-min';

const defaultRow = () => ({ artist: '', title: '', id: '', op: 'ADD', type: 'song' });

// Create Radio Form Component
function CreateRadioForm({ rows, temperature, subtractDistance, onSuccess }) {
  const [radioName, setRadioName] = useState('');
  const [creating, setCreating] = useState(false);
  const [error, setError] = useState('');

  const handleCreateRadio = async () => {
    if (!radioName.trim()) {
      setError('Please enter a radio name');
      return;
    }

    setCreating(true);
    setError('');

    try {
      // Filter rows with valid IDs and create seed songs array
      const seedSongs = rows
        .filter(r => r.id)
        .map(r => ({ id: r.id, op: r.op, type: r.type || 'song' }));

      if (seedSongs.length === 0) {
        setError('No valid songs to save');
        setCreating(false);
        return;
      }

      await createRadio(radioName, seedSongs, temperature, subtractDistance);
      
      // Success! Navigate to radio page
      if (onSuccess) onSuccess();
    } catch (err) {
      setError('Failed to create radio station');
      console.error(err);
    }
    setCreating(false);
  };

  return (
    <div className="space-y-4">
      <h3 className="text-lg font-semibold text-teal-300 flex items-center gap-2">
        <svg className="w-6 h-6" fill="currentColor" viewBox="0 0 20 20">
          <path d="M2 6a2 2 0 012-2h6a2 2 0 012 2v8a2 2 0 01-2 2H4a2 2 0 01-2-2V6zM14.553 7.106A1 1 0 0014 8v4a1 1 0 00.553.894l2 1A1 1 0 0018 13V7a1 1 0 00-1.447-.894l-2 1z"></path>
        </svg>
        Save as Radio Station
      </h3>
      <p className="text-sm text-gray-400">
        Create a radio station from this alchemy recipe. The radio will continuously generate 200 songs at a time using your seed selection.
      </p>
      <div className="flex flex-col sm:flex-row gap-3">
        <input
          type="text"
          placeholder="Enter radio name (e.g., 'Chill Vibes Radio')"
          value={radioName}
          onChange={(e) => setRadioName(e.target.value)}
          className="flex-1 border border-gray-600 bg-gray-900 text-gray-100 rounded-lg px-4 py-2 focus:outline-none focus:border-teal-500 focus:ring-2 focus:ring-teal-500/20"
          disabled={creating}
        />
        <button
          onClick={handleCreateRadio}
          disabled={creating || !radioName.trim()}
          className="border-2 border-teal-500 text-teal-400 bg-teal-500/10 hover:bg-teal-500/20 hover:scale-105 transition-all px-6 py-2 rounded-lg font-semibold disabled:opacity-50 disabled:cursor-not-allowed disabled:hover:scale-100 flex items-center gap-2 justify-center whitespace-nowrap"
        >
          <svg className="w-5 h-5" fill="currentColor" viewBox="0 0 20 20">
            <path fillRule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm1-11a1 1 0 10-2 0v2H7a1 1 0 100 2h2v2a1 1 0 102 0v-2h2a1 1 0 100-2h-2V7z" clipRule="evenodd"></path>
          </svg>
          {creating ? 'Creating...' : 'Create Radio'}
        </button>
      </div>
      {error && <div className="text-red-400 text-sm">{error}</div>}
    </div>
  );
}

export default function SongAlchemy({ onNavigate, onAddToQueue, onPlay, onRadioCreated }) {
  const [rows, setRows] = useState([defaultRow(), defaultRow()]);
  const [nResults, setNResults] = useState(100);
  const [temperature, setTemperature] = useState(1.0);
  const [subtractDistance, setSubtractDistance] = useState(0.3);
  const [results, setResults] = useState(null);
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);
  const [showPreview, setShowPreview] = useState(false);
  const [previewData, setPreviewData] = useState(null);
  const previewPlotRef = useRef(null);

  const handleRowChange = (idx, field, value) => {
    console.log(`handleRowChange: idx=${idx}, field=${field}, value=${value}`);
    setRows(rows => {
      const newRows = rows.map((row, i) => i === idx ? { ...row, [field]: value } : row);
      
      // Trigger suggestions based on the UPDATED row
      const updatedRow = newRows[idx];
      console.log('Updated row:', updatedRow);
      
      if (field === 'artist' || field === 'title') {
        // If this is an artist row typing in artist field, use artist search
        if (updatedRow.type === 'artist' && field === 'artist') {
          console.log('Triggering artist search for idx', idx);
          triggerDebouncedArtistSearch(idx);
        } else if (updatedRow.type === 'song') {
          // For songs, use regular song search
          console.log('Triggering song search for idx', idx);
          triggerDebouncedSearch(idx);
        }
      }
      
      return newRows;
    });
  };

  const triggerDebouncedArtistSearch = (idx) => {
    clearTimeout(timeouts.current[`artist_${idx}`]);
    timeouts.current[`artist_${idx}`] = setTimeout(() => fetchArtistSuggestions(idx), 300);
  };

  // Suggestions per row: { [idx]: [song objects] }
  const [suggestions, setSuggestions] = useState({});
  const [artistSuggestions, setArtistSuggestions] = useState({});
  const timeouts = useRef({});

  const triggerDebouncedSearch = (idx) => {
    clearTimeout(timeouts.current[idx]);
    timeouts.current[idx] = setTimeout(() => fetchSuggestions(idx), 300);
  };

  const fetchSuggestions = async (idx) => {
    const row = rows[idx];
    const q = `${(row.artist || '').trim()} ${(row.title || '').trim()}`.trim();
    if (!q || q.length < 2) {
      setSuggestions(s => ({ ...s, [idx]: [] }));
      return;
    }
    try {
      const data = await searchMusic(q, { songCount: 12 });
      const list = data.searchResult2?.song || data.searchResult3?.song || [];
      const arr = Array.isArray(list) ? list : [list].filter(Boolean);
      setSuggestions(s => ({ ...s, [idx]: arr }));
    } catch (e) {
      console.error('Autocomplete search failed', e);
      setSuggestions(s => ({ ...s, [idx]: [] }));
    }
  };

  const fetchArtistSuggestions = async (idx) => {
    const row = rows[idx];
    const q = (row.artist || '').trim();
    console.log(`fetchArtistSuggestions for idx=${idx}, query="${q}", row.type="${row.type}"`);
    if (!q || q.length < 2) {
      setArtistSuggestions(s => ({ ...s, [idx]: [] }));
      return;
    }
    try {
      const token = localStorage.getItem('token');
      const url = `/api/alchemy/search_artists?query=${encodeURIComponent(q)}`;
      console.log('Fetching artist suggestions from:', url);
      const resp = await fetch(url, {
        headers: token ? { 'Authorization': `Bearer ${token}` } : {}
      });
      const data = await resp.json();
      console.log('Artist suggestions received:', data);
      setArtistSuggestions(s => ({ ...s, [idx]: data || [] }));
    } catch (e) {
      console.error('Artist autocomplete search failed', e);
      setArtistSuggestions(s => ({ ...s, [idx]: [] }));
    }
  };

  const selectSuggestion = (idx, item) => {
    setRows(rs => rs.map((r, i) => i === idx ? { ...r, id: item.id || item.item_id || item.songId || item.id, artist: item.artist || item.author || item.creator || r.artist, title: item.title || r.title, op: r.op, type: 'song' } : r));
    setSuggestions(s => ({ ...s, [idx]: [] }));
  };

  const selectArtistSuggestion = (idx, artist) => {
    console.log('Selected artist:', artist, 'ID:', artist.artist_id);
    setRows(rs => rs.map((r, i) => {
      if (i === idx) {
        const updated = { ...r, id: artist.artist_id || artist.artist, artist: artist.artist, title: '', op: r.op, type: 'artist' };
        console.log('Row updated to:', updated);
        return updated;
      }
      return r;
    }));
    setArtistSuggestions(s => ({ ...s, [idx]: [] }));
  };

  const addRow = () => setRows([...rows, defaultRow()]);
  const removeRow = idx => setRows(rows => rows.filter((_, i) => i !== idx));

  const handlePreview = async () => {
    setError('');
    setLoading(true);
    try {
      const rowsCopy = [...rows];
      console.log('Rows before filtering:', rowsCopy);
      for (let i = 0; i < rowsCopy.length; i++) {
        const r = rowsCopy[i];
        // Auto-resolve logic only for songs - artists should already have IDs from selection
        if (!r.id && r.type === 'song') {
          const q = `${(r.artist || '').trim()} ${(r.title || '').trim()}`.trim();
          if (q && q.length >= 3) {
            try {
              const data = await searchMusic(q, { songCount: 5 });
              const list = data.searchResult2?.song || data.searchResult3?.song || [];
              const arr = Array.isArray(list) ? list : [list].filter(Boolean);
              if (arr.length === 1) {
                const item = arr[0];
                rowsCopy[i] = { ...r, id: item.id || item.item_id };
              }
            } catch (err) {
              console.warn('Auto-resolve search failed for row', i, err);
            }
          }
        }
      }
      const items = rowsCopy.filter(r => r.id).map(r => ({ id: r.id, op: r.op, type: r.type || 'song' }));
      console.log('Items after filtering:', items);
      console.log('Payload:', JSON.stringify({ items, n: nResults, temperature, subtract_distance: subtractDistance, preview: true }, null, 2));
      if (!items.some(i => i.op === 'ADD')) {
        setError('Please include at least one ADD item.');
        setLoading(false);
        return;
      }
      const payload = {
        items,
        n: nResults,
        temperature,
        subtract_distance: subtractDistance,
        preview: true
      };
      const token = localStorage.getItem('token');
      const resp = await fetch('/api/alchemy', {
        method: 'POST',
        headers: token ? { 'Content-Type': 'application/json', 'Authorization': `Bearer ${token}` } : { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload)
      });
      const data = await resp.json();
      if (!resp.ok || data.error) {
        setError(data.error || 'Preview failed');
        setLoading(false);
        return;
      }
      setPreviewData(data);
      setShowPreview(true);
    } catch (err) {
      setError('Preview request failed');
    }
    setLoading(false);
  };

  useEffect(() => {
    if (showPreview && previewData && previewPlotRef.current) {
      const fullLabel = (item) => (item.title ? `${item.title} — ${item.author || item.artist || ''}` : item.item_id);
      
      const traces = [];
      
      // Removed (filtered out) - Gray X
      if (previewData.filtered_out && previewData.filtered_out.length) {
        traces.push({
          x: previewData.filtered_out.map(p => p.embedding_2d ? p.embedding_2d[0] : 0),
          y: previewData.filtered_out.map(p => p.embedding_2d ? p.embedding_2d[1] : 0),
          text: previewData.filtered_out.map(fullLabel),
          mode: 'markers',
          type: 'scatter',
          name: 'Removed (filtered out)',
          marker: { size: 5, color: 'rgba(156, 163, 175, 0.5)', symbol: 'x', line: { width: 2 } }
        });
      }
      
      // Kept (results) - Blue circles
      if (previewData.results && previewData.results.length) {
        traces.push({
          x: previewData.results.map(p => p.embedding_2d ? p.embedding_2d[0] : 0),
          y: previewData.results.map(p => p.embedding_2d ? p.embedding_2d[1] : 0),
          text: previewData.results.map(fullLabel),
          mode: 'markers',
          type: 'scatter',
          name: 'Kept (results)',
          marker: { size: 6, color: 'rgba(59, 130, 246, 0.7)', line: { width: 1, color: 'rgba(37, 99, 235, 1)' } }
        });
      }
      
      // ADD Centroid - Green triangle
      if (previewData.add_centroid_2d) {
        traces.push({
          x: [previewData.add_centroid_2d[0]],
          y: [previewData.add_centroid_2d[1]],
          text: ['ADD Centroid'],
          mode: 'markers',
          type: 'scatter',
          name: 'Add Centroid',
          marker: { size: 20, color: 'rgba(34, 197, 94, 1)', symbol: 'triangle-up', line: { width: 3, color: 'rgba(22, 163, 74, 1)' } }
        });
      }
      
      // SUBTRACT Centroid - Red triangle
      if (previewData.subtract_centroid_2d) {
        traces.push({
          x: [previewData.subtract_centroid_2d[0]],
          y: [previewData.subtract_centroid_2d[1]],
          text: ['SUBTRACT Centroid'],
          mode: 'markers',
          type: 'scatter',
          name: 'Subtract Centroid',
          marker: { size: 20, color: 'rgba(239, 68, 68, 1)', symbol: 'triangle-down', line: { width: 3, color: 'rgba(220, 38, 38, 1)' } }
        });
      }
      
      // Selected ADD songs - Green circles
      if (previewData.add_points && previewData.add_points.length) {
        const addSongs = previewData.add_points.filter(p => !p.is_artist_component);
        const addArtistComponents = previewData.add_points.filter(p => p.is_artist_component);
        
        if (addSongs.length > 0) {
          traces.push({
            x: addSongs.map(p => p.embedding_2d ? p.embedding_2d[0] : 0),
            y: addSongs.map(p => p.embedding_2d ? p.embedding_2d[1] : 0),
            text: addSongs.map(fullLabel),
            mode: 'markers',
            type: 'scatter',
            name: 'Selected ADD Songs',
            marker: { size: 12, color: 'rgba(34, 197, 94, 1)', symbol: 'circle', line: { width: 2, color: 'rgba(22, 163, 74, 1)' } }
          });
        }
        
        if (addArtistComponents.length > 0) {
          traces.push({
            x: addArtistComponents.map(p => p.embedding_2d ? p.embedding_2d[0] : 0),
            y: addArtistComponents.map(p => p.embedding_2d ? p.embedding_2d[1] : 0),
            text: addArtistComponents.map(fullLabel),
            mode: 'markers',
            type: 'scatter',
            name: 'Selected ADD Artist Components',
            marker: { size: 10, color: 'rgba(34, 197, 94, 1)', symbol: 'square', line: { width: 2, color: 'rgba(22, 163, 74, 1)' } }
          });
        }
      }
      
      // Selected SUBTRACT songs - Red circles
      if (previewData.sub_points && previewData.sub_points.length) {
        const subSongs = previewData.sub_points.filter(p => !p.is_artist_component);
        const subArtistComponents = previewData.sub_points.filter(p => p.is_artist_component);
        
        if (subSongs.length > 0) {
          traces.push({
            x: subSongs.map(p => p.embedding_2d ? p.embedding_2d[0] : 0),
            y: subSongs.map(p => p.embedding_2d ? p.embedding_2d[1] : 0),
            text: subSongs.map(fullLabel),
            mode: 'markers',
            type: 'scatter',
            name: 'Selected SUBTRACT Songs',
            marker: { size: 12, color: 'rgba(239, 68, 68, 1)', symbol: 'circle', line: { width: 2, color: 'rgba(220, 38, 38, 1)' } }
          });
        }
        
        if (subArtistComponents.length > 0) {
          traces.push({
            x: subArtistComponents.map(p => p.embedding_2d ? p.embedding_2d[0] : 0),
            y: subArtistComponents.map(p => p.embedding_2d ? p.embedding_2d[1] : 0),
            text: subArtistComponents.map(fullLabel),
            mode: 'markers',
            type: 'scatter',
            name: 'Selected SUBTRACT Artist Components',
            marker: { size: 10, color: 'rgba(239, 68, 68, 1)', symbol: 'square', line: { width: 2, color: 'rgba(220, 38, 38, 1)' } }
          });
        }
      }
      
      const layout = {
        hovermode: 'closest',
        legend: { orientation: 'h', y: -0.2 },
        margin: { t: 20, b: 40, l: 40, r: 20 },
        paper_bgcolor: '#1f2937',
        plot_bgcolor: '#111827',
        font: { color: '#d1d5db' },
        xaxis: { title: 'Dimension 1', gridcolor: '#374151' },
        yaxis: { title: 'Dimension 2', gridcolor: '#374151' }
      };
      
      Plotly.newPlot(previewPlotRef.current, traces, layout, { responsive: true });
    }
  }, [showPreview, previewData]);

  const handleSubmit = async e => {
    e.preventDefault();
    setError('');
    setLoading(true);
    try {
      // Attempt to auto-resolve rows that have artist/title but no id
      const rowsCopy = [...rows];
      for (let i = 0; i < rowsCopy.length; i++) {
        const r = rowsCopy[i];
        if (!r.id) {
          const q = `${(r.artist || '').trim()} ${(r.title || '').trim()}`.trim();
          if (q && q.length >= 3) {
            try {
              const data = await searchMusic(q, { songCount: 5 });
              const list = data.searchResult2?.song || data.searchResult3?.song || [];
              const arr = Array.isArray(list) ? list : [list].filter(Boolean);
              if (arr.length === 1) {
                // auto-select unique match
                const item = arr[0];
                rowsCopy[i] = { ...r, id: item.id || item.item_id };
              }
            } catch (err) {
              console.warn('Auto-resolve search failed for row', i, err);
            }
          }
        }
      }
      const items = rowsCopy.filter(r => r.id).map(r => ({ id: r.id, op: r.op, type: r.type || 'song' }));
      if (!items.some(i => i.op === 'ADD')) {
        setError('Please include at least one ADD song.');
        setLoading(false);
        return;
      }
      const payload = {
        items,
        n: nResults,
        temperature,
        subtract_distance: subtractDistance
      };
      const token = localStorage.getItem('token');
      const resp = await fetch('/api/alchemy', {
        method: 'POST',
        headers: token ? { 'Content-Type': 'application/json', 'Authorization': `Bearer ${token}` } : { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload)
      });
      const data = await resp.json();
      if (!resp.ok || data.error) {
        setError(data.error || 'Alchemy failed');
        setLoading(false);
        return;
      }

      // Map results to song objects expected by Songs view (id, title, artist)
      const mapped = (data.results || []).map(r => ({
        id: r.item_id || r.id || r.songId || '',
        title: r.title || r.name || '',
        artist: r.author || r.artist || r.creator || ''
      }));

      // If caller provided onNavigate, navigate to Songs with preloadedSongs
      if (onNavigate) {
        onNavigate({ page: 'songs', title: `Alchemy: Results`, filter: { preloadedSongs: mapped } });
      } else {
        // Fallback: set results to show table in this component
        setResults(data);
      }
    } catch (err) {
      setError('Request failed');
    }
    setLoading(false);
  };

  const handlePlayNow = async () => {
    setError('');
    setLoading(true);
    try {
      // Attempt to auto-resolve rows that have artist/title but no id
      const rowsCopy = [...rows];
      for (let i = 0; i < rowsCopy.length; i++) {
        const r = rowsCopy[i];
        if (!r.id) {
          const q = `${(r.artist || '').trim()} ${(r.title || '').trim()}`.trim();
          if (q && q.length >= 3) {
            try {
              const data = await searchMusic(q, { songCount: 5 });
              const list = data.searchResult2?.song || data.searchResult3?.song || [];
              const arr = Array.isArray(list) ? list : [list].filter(Boolean);
              if (arr.length === 1) {
                const item = arr[0];
                rowsCopy[i] = { ...r, id: item.id || item.item_id };
              }
            } catch (err) {
              console.warn('Auto-resolve search failed for row', i, err);
            }
          }
        }
      }
      const items = rowsCopy.filter(r => r.id).map(r => ({ id: r.id, op: r.op, type: r.type || 'song' }));
      if (!items.some(i => i.op === 'ADD')) {
        setError('Please include at least one ADD song.');
        setLoading(false);
        return;
      }
      const payload = {
        items,
        n: 200,  // Always 200 for Play Now
        temperature,
        subtract_distance: subtractDistance
      };
      const resp = await fetch('/api/alchemy', {
        method: 'POST',
        headers: { 
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${localStorage.getItem('token')}`
        },
        body: JSON.stringify(payload)
      });
      const data = await resp.json();
      if (!resp.ok || data.error) {
        setError(data.error || 'Alchemy failed');
        setLoading(false);
        return;
      }

      // Map results to song objects WITHOUT radio metadata (so it won't auto-rerun)
      const mapped = (data.results || []).map(r => ({
        id: r.item_id || r.id || r.songId || '',
        title: r.title || r.name || '',
        artist: r.author || r.artist || r.creator || ''
        // NO _radioId, _radioName, _isRadioSong - this is NOT a radio, just alchemy results
      }));

      // Navigate to song list (old implementation) - shows the list, doesn't auto-rerun
      if (onNavigate) {
        onNavigate({ page: 'songs', title: `Alchemy: Play Now Results`, filter: { preloadedSongs: mapped } });
        setShowPreview(false);  // Close preview after navigating
      }
    } catch (err) {
      setError('Request failed: ' + err.message);
    }
    setLoading(false);
  };

  return (
    <div className="text-gray-100">
      <p className="mb-4 text-gray-300">Select tracks or artists to Include or Exclude — boost favorites with Include and remove unwanted flavors with Exclude.</p>
      <form onSubmit={handleSubmit} className="space-y-4">
        <fieldset className="border border-gray-700 rounded p-4 bg-gray-800">
          <legend className="font-semibold text-teal-300">Artist and/or Song Selection</legend>
          {rows.map((row, idx) => (
            <div key={idx} className="flex flex-col gap-2 mb-4 p-3 border border-gray-600 rounded bg-gray-900">
              <div className="flex flex-wrap items-center gap-2">
                {/* Type Toggle Button */}
                <button
                  type="button"
                  onClick={() => {
                    const newType = row.type === 'song' ? 'artist' : 'song';
                    console.log(`Toggle type for idx=${idx}: ${row.type} -> ${newType}`);
                    setRows(rows => {
                      const newRows = rows.map((r, i) => {
                        if (i === idx) {
                          if (newType === 'artist') {
                            // Switching to artist mode - keep artist name, clear ID and title
                            const updated = { ...r, type: newType, id: '', title: '' };
                            console.log('Row after type toggle to artist:', updated);
                            // Trigger artist search if we have an artist name
                            if (updated.artist && updated.artist.trim().length >= 2) {
                              console.log('Auto-triggering artist search after toggle');
                              setTimeout(() => triggerDebouncedArtistSearch(idx), 100);
                            }
                            return updated;
                          } else {
                            // Switching to song mode - keep artist name, clear ID, keep title if exists
                            const updated = { ...r, type: newType, id: '' };
                            console.log('Row after type toggle to song:', updated);
                            return updated;
                          }
                        }
                        return r;
                      });
                      return newRows;
                    });
                  }}
                  className={`px-3 py-1.5 rounded-lg font-semibold text-sm transition-all ${
                    row.type === 'artist' 
                      ? 'bg-purple-500/20 border-2 border-purple-500 text-purple-400 hover:bg-purple-500/30' 
                      : 'bg-blue-500/20 border-2 border-blue-500 text-blue-400 hover:bg-blue-500/30'
                  }`}
                  title="Toggle Song/Artist"
                >
                  {row.type === 'artist' ? '🎤 Artist' : '🎵 Song'}
                </button>

                {/* Operation Selector */}
                <select 
                  value={row.op} 
                  onChange={e => handleRowChange(idx, 'op', e.target.value)} 
                  className="border border-gray-600 bg-gray-900 text-gray-100 rounded px-2 py-1"
                >
                  <option value="ADD">Include</option>
                  <option value="SUBTRACT">Exclude</option>
                </select>

                {/* Remove Button */}
                <button 
                  type="button" 
                  onClick={() => removeRow(idx)} 
                  className="ml-auto text-red-400 hover:text-red-200 text-2xl font-bold px-2"
                >
                  &times;
                </button>
              </div>

              {/* Artist Field */}
              <div className="relative">
                <input 
                  type="text" 
                  placeholder="Artist" 
                  value={row.artist} 
                  onChange={e => handleRowChange(idx, 'artist', e.target.value)} 
                  className="border border-gray-600 bg-gray-900 text-gray-100 rounded px-3 py-2 w-full"
                />
                {/* Artist Suggestions (only show in artist mode) */}
                {row.type === 'artist' && artistSuggestions[idx] && artistSuggestions[idx].length > 0 && (
                  <div className="absolute left-0 right-0 mt-1 bg-gray-800 border border-gray-700 rounded z-50 max-h-48 overflow-auto">
                    {artistSuggestions[idx].map((artist, si) => (
                      <div 
                        key={si} 
                        onMouseDown={() => selectArtistSuggestion(idx, artist)} 
                        className="px-3 py-2 hover:bg-gray-700 cursor-pointer text-sm"
                      >
                        <div className="font-semibold">{artist.artist}</div>
                        <div className="text-gray-400 text-xs">{artist.track_count} tracks</div>
                      </div>
                    ))}
                  </div>
                )}
              </div>

              {/* Title Field (only for songs) */}
              {row.type === 'song' && (
                <div className="relative">
                  <input 
                    type="text" 
                    placeholder="Title" 
                    value={row.title} 
                    onChange={e => handleRowChange(idx, 'title', e.target.value)} 
                    className="border border-gray-600 bg-gray-900 text-gray-100 rounded px-3 py-2 w-full"
                  />
                  {/* Song Suggestions */}
                  {suggestions[idx] && suggestions[idx].length > 0 && (
                    <div className="absolute left-0 right-0 mt-1 bg-gray-800 border border-gray-700 rounded z-50 max-h-48 overflow-auto">
                      {suggestions[idx].map((s, si) => (
                        <div 
                          key={si} 
                          onMouseDown={() => selectSuggestion(idx, s)} 
                          className="px-3 py-2 hover:bg-gray-700 cursor-pointer text-sm"
                        >
                          <div className="font-semibold">{s.title}</div>
                          <div className="text-gray-400 text-xs">{s.artist || s.author}</div>
                        </div>
                      ))}
                    </div>
                  )}
                </div>
              )}

              {/* Selected Item Display */}
              {row.id ? (
                <div className="text-xs text-teal-400 mt-1 font-semibold">
                  ✓ Selected: {row.type === 'artist' ? `${row.artist} (ID: ${row.id})` : `${row.title} — ${row.artist} (ID: ${row.id})`}
                </div>
              ) : (
                <div className="text-xs text-yellow-400 mt-1">
                  ⚠ Not selected yet - {row.type === 'artist' ? 'type artist name and select from dropdown' : 'type song details and select from dropdown'}
                </div>
              )}
            </div>
          ))}
          <button 
            type="button" 
            onClick={addRow} 
            className="border-2 border-blue-500 text-blue-400 bg-blue-500/10 hover:bg-blue-500/20 hover:scale-105 transition-all px-3 py-1.5 rounded-lg"
          >
            Add Another Item
          </button>
        </fieldset>
        <fieldset className="border border-gray-700 rounded p-4 bg-gray-800">
          <legend className="font-semibold text-teal-300">Parameters</legend>
          <div className="flex gap-4">
            <div>
              <label className="block text-gray-300">Number of results:</label>
              <input type="number" min={1} max={200} value={nResults} onChange={e => setNResults(Number(e.target.value))} className="border border-gray-600 bg-gray-900 text-gray-100 rounded px-2 py-1 w-20" />
            </div>
            <div>
              <label className="block text-gray-300">Sampling temperature (τ):</label>
              <input type="number" step={0.1} min={0} max={10} value={temperature} onChange={e => setTemperature(Number(e.target.value))} className="border border-gray-600 bg-gray-900 text-gray-100 rounded px-2 py-1 w-20" />
            </div>
            <div>
              <label className="block text-gray-300">Subtract distance threshold:</label>
              <input type="number" step={0.01} min={0} max={1} value={subtractDistance} onChange={e => setSubtractDistance(Number(e.target.value))} className="border border-gray-600 bg-gray-900 text-gray-100 rounded px-2 py-1 w-20" />
            </div>
          </div>
        </fieldset>
        <div className="flex gap-3">
          <button type="button" onClick={handlePreview} className="border-2 border-blue-500 text-blue-400 bg-blue-500/10 hover:bg-blue-500/20 hover:scale-105 transition-all px-6 py-2 rounded-lg font-semibold disabled:opacity-50 disabled:cursor-not-allowed disabled:hover:scale-100" disabled={loading}>{loading ? 'Loading...' : 'Preview'}</button>
        </div>
      </form>
      {error && <div className="text-red-400 mt-2">{error}</div>}
      
      {showPreview && previewData && (
        <div className="mt-6 border border-gray-700 rounded-lg p-4 bg-gray-800">
          <div className="flex justify-between items-center mb-4">
            <h2 className="text-xl font-semibold text-teal-300">Preview Map</h2>
            <button onClick={() => setShowPreview(false)} className="text-gray-400 hover:text-white text-2xl">&times;</button>
          </div>
          <div ref={previewPlotRef} style={{ width: '100%', height: '500px' }}></div>
          <div className="text-center text-sm text-gray-400 mt-2">
            Projection Method: {previewData.projection || 'pca'}
          </div>
          
          {/* Action Buttons */}
          <div className="mt-6 border-t border-gray-700 pt-6 space-y-6">
            {/* Create Radio Station - First Choice */}
            <CreateRadioForm 
              rows={rows}
              temperature={temperature}
              subtractDistance={subtractDistance}
              onSuccess={() => {
                setShowPreview(false);
                if (onRadioCreated) onRadioCreated();
              }}
            />

            {/* OR Divider */}
            <div className="flex items-center gap-4">
              <div className="flex-1 border-t border-gray-600"></div>
              <span className="text-gray-500 font-semibold">OR</span>
              <div className="flex-1 border-t border-gray-600"></div>
            </div>

            {/* Play Now - Second Choice */}
            <div className="text-center">
              <button
                onClick={handlePlayNow}
                disabled={loading}
                className="border-2 border-green-500 text-green-400 bg-green-500/10 hover:bg-green-500/20 hover:scale-105 transition-all px-8 py-3 rounded-lg font-semibold inline-flex items-center gap-2 disabled:opacity-50 disabled:cursor-not-allowed disabled:hover:scale-100"
              >
                {loading ? (
                  <>
                    <svg className="animate-spin h-6 w-6" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
                      <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"></circle>
                      <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                    </svg>
                    Generating...
                  </>
                ) : (
                  <>
                    <svg className="w-6 h-6" fill="currentColor" viewBox="0 0 20 20">
                      <path d="M6.3 2.841A1.5 1.5 0 004 4.11V15.89a1.5 1.5 0 002.3 1.269l9.344-5.89a1.5 1.5 0 000-2.538L6.3 2.84z"></path>
                    </svg>
                    Play Now (200 songs)
                  </>
                )}
              </button>
              <p className="text-gray-500 text-sm mt-2">Start playing immediately without saving</p>
            </div>
          </div>
        </div>
      )}
      
      {results && (
        <div className="mt-6">
          <h2 className="text-xl font-semibold mb-2 text-teal-300">Results</h2>
          <table className="min-w-full border border-gray-700 bg-gray-800 text-gray-100">
            <thead>
              <tr>
                <th className="border border-gray-700 px-2 py-1">Title</th>
                <th className="border border-gray-700 px-2 py-1">Artist</th>
                <th className="border border-gray-700 px-2 py-1">Distance</th>
              </tr>
            </thead>
            <tbody>
              {results.results && results.results.length > 0 ? results.results.map((r, i) => (
                <tr key={i}>
                  <td className="border border-gray-700 px-2 py-1">{r.title}</td>
                  <td className="border border-gray-700 px-2 py-1">{r.author}</td>
                  <td className="border border-gray-700 px-2 py-1">{typeof r.distance === 'number' ? r.distance.toFixed(4) : 'N/A'}</td>
                </tr>
              )) : <tr><td colSpan={3} className="text-center">No results found.</td></tr>}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
