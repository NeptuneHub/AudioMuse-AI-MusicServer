import React, { useState, useRef } from 'react';
import { searchMusic } from '../api';

const defaultRow = () => ({ artist: '', title: '', id: '', op: 'ADD' });

export default function SongAlchemy({ onNavigate, onAddToQueue, onPlay }) {
  const [rows, setRows] = useState([defaultRow(), defaultRow()]);
  const [nResults, setNResults] = useState(100);
  const [temperature, setTemperature] = useState(1.0);
  const [subtractDistance, setSubtractDistance] = useState(0.3);
  const [results, setResults] = useState(null);
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);

  const handleRowChange = (idx, field, value) => {
    setRows(rows => rows.map((row, i) => i === idx ? { ...row, [field]: value } : row));
    // Trigger suggestions when artist/title change
    if (field === 'artist' || field === 'title') {
      triggerDebouncedSearch(idx);
    }
  };

  // Suggestions per row: { [idx]: [song objects] }
  const [suggestions, setSuggestions] = useState({});
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

  const selectSuggestion = (idx, item) => {
    setRows(rs => rs.map((r, i) => i === idx ? { ...r, id: item.id || item.item_id || item.songId || item.id, artist: item.artist || item.author || item.creator || r.artist, title: item.title || r.title, op: r.op } : r));
    setSuggestions(s => ({ ...s, [idx]: [] }));
  };

  const addRow = () => setRows([...rows, defaultRow()]);
  const removeRow = idx => setRows(rows => rows.filter((_, i) => i !== idx));

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
      const items = rowsCopy.filter(r => r.id).map(r => ({ id: r.id, op: r.op }));
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
      const resp = await fetch('/api/alchemy', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
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

  return (
    <div className="text-gray-100">
      <p className="mb-4 text-gray-300">Select tracks to Include or Exclude — boost favorites with Include and remove unwanted flavors with Exclude.</p>
      <form onSubmit={handleSubmit} className="space-y-4">
        <fieldset className="border border-gray-700 rounded p-4 bg-gray-800">
          <legend className="font-semibold text-teal-300">Song Selection</legend>
          {rows.map((row, idx) => (
            <div key={idx} className="flex flex-col sm:flex-row sm:items-center gap-2 mb-2 w-full">
              <div className="flex-shrink-0">
                <select value={row.op} onChange={e => handleRowChange(idx, 'op', e.target.value)} className="border border-gray-600 bg-gray-900 text-gray-100 rounded px-2 py-1">
                  <option value="ADD">Include</option>
                  <option value="SUBTRACT">Exclude</option>
                </select>
              </div>
              <input type="text" placeholder="Artist" value={row.artist} onChange={e => handleRowChange(idx, 'artist', e.target.value)} className="border border-gray-600 bg-gray-900 text-gray-100 rounded px-2 py-1 w-full sm:w-48" />
              <div className="relative w-full sm:w-64">
                <input type="text" placeholder="Title" value={row.title} onChange={e => handleRowChange(idx, 'title', e.target.value)} className="border border-gray-600 bg-gray-900 text-gray-100 rounded px-2 py-1 w-full" />
                {suggestions[idx] && suggestions[idx].length > 0 && (
                  <div className="absolute left-0 right-0 mt-1 bg-gray-800 border border-gray-700 rounded z-50 max-h-48 overflow-auto">
                    {suggestions[idx].map((s, si) => (
                      <div key={si} onMouseDown={() => selectSuggestion(idx, s)} className="px-3 py-2 hover:bg-gray-700 cursor-pointer text-sm">
                        <div className="font-semibold">{s.title}</div>
                        <div className="text-gray-400 text-xs">{s.artist || s.author}</div>
                      </div>
                    ))}
                  </div>
                )}
              </div>
              <div className="flex-shrink-0">
                <button type="button" onClick={() => removeRow(idx)} className="text-red-400 hover:text-red-200 text-xl">&times;</button>
              </div>
            </div>
          ))}
          <button type="button" onClick={addRow} className="bg-blue-600 hover:bg-blue-700 text-white px-3 py-1 rounded">Add Another Song</button>
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
        <button type="submit" className="bg-green-600 hover:bg-green-700 text-white px-6 py-2 rounded font-semibold" disabled={loading}>{loading ? 'Running...' : 'Run Alchemy'}</button>
      </form>
      {error && <div className="text-red-400 mt-2">{error}</div>}
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
