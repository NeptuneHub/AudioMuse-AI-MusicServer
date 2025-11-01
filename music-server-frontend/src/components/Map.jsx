import React, { useEffect, useState, useRef, useCallback } from 'react';
import Plotly from 'plotly.js-dist-min';
import { apiFetch, searchMusic, subsonicFetch } from '../api';

export default function Map({ onNavigate, onAddToQueue, onPlay, onRemoveFromQueue, onClearQueue, playQueue = [] }) {
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [selectedIds, setSelectedIds] = useState([]);
  const [percent, setPercent] = useState(25);
  const [query, setQuery] = useState('');
  const [suggestions, setSuggestions] = useState([]);
  const [hiddenGenres, setHiddenGenres] = useState(new Set());
  const [genres, setGenres] = useState([]);
  const [showSearchHighlight, setShowSearchHighlight] = useState(true);
  const [showPathLine, setShowPathLine] = useState(true);
  const [showPathPoints, setShowPathPoints] = useState(true);
  const fetchRef = useRef(0);
  const plotDivRef = useRef(null);
  const rawItemsRef = useRef([]); // Store original items for re-rendering

  // helper to extract ids (ported from map.html)
  const extractIdsFromPoints = useCallback((points) => {
    const ids = [];
    if (!points) return ids;
    for (const p of points) {
      if (p.customdata !== undefined && p.customdata !== null) { ids.push(String(p.customdata)); continue; }
      const idx = (p.pointIndex !== undefined) ? p.pointIndex : ((p.pointNumber !== undefined) ? p.pointNumber : (p.index !== undefined ? p.index : null));
      if (idx !== null) {
        if (p.data && p.data.customdata && p.data.customdata[idx] !== undefined && p.data.customdata[idx] !== null) { ids.push(String(p.data.customdata[idx])); continue; }
        if (window._plotPoints && window._plotPoints[idx] && window._plotPoints[idx].id !== undefined) { ids.push(String(window._plotPoints[idx].id)); continue; }
        try { const gd = document.getElementById('map-plot'); if (gd && gd.data && gd.data[0] && gd.data[0].customdata && gd.data[0].customdata[idx] !== undefined) { ids.push(String(gd.data[0].customdata[idx])); continue; } } catch(e) {}
      }
    }
    if (ids.length === 0) console.debug('extractIdsFromPoints produced no ids for points', points);
    return ids;
  }, []);

  // Sync selectedIds when playQueue changes (bidirectional sync)
  useEffect(() => {
    if (!playQueue) return;
    
    // Get IDs currently in the queue
    const queueIds = playQueue.map(song => String(song.id));
    const queueIdSet = new Set(queueIds);
    
    // Remove from selectedIds any IDs that are not in the queue
    const currentSelection = window._plotSelection || [];
    const newSelection = currentSelection.filter(id => queueIdSet.has(id));
    
    if (newSelection.length !== currentSelection.length) {
      window._plotSelection = newSelection;
      setSelectedIds(newSelection);
      console.log('Queue sync: removed', currentSelection.length - newSelection.length, 'deselected songs from map');
    }
  }, [playQueue]);

  const attachPlotHandlers = useCallback((gd) => {
    if (!gd) return;
    console.log('attachPlotHandlers called on element:', gd?.id, 'typeof gd.on:', typeof gd?.on);
    try { if (gd._amy_handlers_attached) { /* allow reattach */ } } catch(e){}
    
    // Match the working HTML version binding order: gd.on() first, then addEventListener, then Plotly.on
    const bind = (type, fn) => { 
      try { 
        if (typeof gd.on === 'function') {
          console.log('Binding', type, 'using gd.on()');
          gd.on(type, fn);
        } else if (typeof gd.addEventListener === 'function') {
          console.log('Binding', type, 'using addEventListener');
          gd.addEventListener(type, fn);
        } else if (Plotly && typeof Plotly.on === 'function') {
          console.log('Binding', type, 'using Plotly.on()');
          Plotly.on(gd, type, fn);
        } else {
          console.warn('No supported event binding method found for', type);
        }
      } catch(e){ console.warn('bind failed for', type, e); } 
    };
    
    bind('plotly_selected', (ev) => {
      console.log('plotly_selected event fired, points:', ev?.points?.length || 0);
      const ids = extractIdsFromPoints(ev && ev.points);
      console.log('extractIdsFromPoints returned:', ids.length, 'ids:', ids);
      if (!ids || ids.length === 0) return;
      window._plotSelection = window._plotSelection || [];
      const beforeLen = window._plotSelection.length;
      const existingSet = new Set(window._plotSelection.map(String));
      const MAX_ADD = 1000; let added = 0; const space = Math.max(0, MAX_ADD - window._plotSelection.length);
      for (const id of ids) { if (added >= space) break; const sid = String(id); if (!existingSet.has(sid)) { window._plotSelection.push(sid); existingSet.add(sid); added++; } }
      console.log('Selection updated: before=', beforeLen, 'after=', window._plotSelection.length, 'added=', added);
      // Dispatch event to update React state (don't manually update DOM)
      try { window.dispatchEvent(new CustomEvent('am-map-selection-changed')); } catch(e){}
    });

    bind('plotly_click', async (ev) => {
      console.log('plotly_click event fired, point:', ev?.points?.[0]);
      const pt = ev && ev.points && ev.points[0];
      const ids = extractIdsFromPoints(pt ? [pt] : []);
      if (!ids || ids.length === 0) return;
      const id = String(ids[0]);
      
      // Debounce: Plotly fires click twice, prevent double-toggle
      const now = Date.now();
      const lastClick = gd._lastClickTime || 0;
      const lastClickId = gd._lastClickId || null;
      if (lastClickId === id && now - lastClick < 300) {
        console.log('Ignoring duplicate click event for song', id);
        return;
      }
      gd._lastClickTime = now;
      gd._lastClickId = id;
      
      window._plotSelection = window._plotSelection || [];
      
      // Toggle selection: if already selected, remove it (and from queue); otherwise add it (and to queue)
      const idx = window._plotSelection.indexOf(id);
      if (idx !== -1) {
        window._plotSelection.splice(idx, 1);
        console.log('Deselected song', id);
        // Remove from queue if onRemoveFromQueue is available
        if (onRemoveFromQueue) {
          onRemoveFromQueue(id);
        }
      } else {
        window._plotSelection.push(id);
        console.log('Selected song', id);
        
        // Add to queue - fetch song details first
        if (onAddToQueue) {
          try {
            const data = await subsonicFetch('getSong.view', { id });
            if (data && data.song) {
              // Set flag before adding to queue so AudioPlayer won't auto-play
              window._mapAddedSong = true;
              onAddToQueue(data.song);
              console.log('Added song to queue from Map:', data.song.title);
              
              // Safety: clear flag after a short delay in case AudioPlayer doesn't clear it
              setTimeout(() => {
                if (window._mapAddedSong) {
                  window._mapAddedSong = false;
                  console.log('Map: Auto-cleared _mapAddedSong flag after timeout');
                }
              }, 1000);
            }
          } catch (err) {
            console.warn('Failed to fetch song for queue:', err);
          }
        }
      }
      
      // Dispatch event to update React state (don't manually update DOM)
      try { window.dispatchEvent(new CustomEvent('am-map-selection-changed')); } catch(e){}
    });

    gd._amy_handlers_attached = true;
    console.log('attachPlotHandlers completed - handlers bound to plotly_selected and plotly_click');
  }, [extractIdsFromPoints, onAddToQueue, onRemoveFromQueue]);

  // renderPlot: builds traces and calls Plotly.react/newPlot
  const renderPlot = useCallback((items, projection) => {
    const gd = plotDivRef.current || document.getElementById('map-plot');
    if (!gd) {
      console.warn('renderPlot: plot div not found! plotDivRef.current=', plotDivRef.current, 'getElementById=', document.getElementById('map-plot'));
      return;
    }
    console.log('renderPlot: gd element found, dimensions:', gd.offsetWidth, 'x', gd.offsetHeight, 'style:', gd.style.cssText);
    try { if (gd.layout && gd.layout.selections) delete gd.layout.selections; } catch(e) {}

    const pts = [];
    const genres = new Set();
    for (const it of items) {
      // Accept embedding_2d as an array or as a comma-separated string (defensive)
      let e = it.embedding_2d;
      if (!e || (Array.isArray(e) && e.length < 2)) {
        // try other common fields or skip
        e = it.embedding || it.coords || it.embedding2d || e;
      }
      if (typeof e === 'string') {
        // parse "x,y"
        const parts = e.split(',').map(s => parseFloat(s.trim())).filter(n => !Number.isNaN(n));
        if (parts.length >= 2) e = parts;
      }
      if (!e || !Array.isArray(e) || e.length < 2) continue;
      const genre = (it.mood_vector && typeof it.mood_vector === 'string') ? (it.mood_vector.split(',')[0]?.split(':')[0] || 'unknown') : 'unknown';
      genres.add(genre);
      pts.push({ id: String(it.item_id || it.id || ''), x: e[0], y: e[1], title: it.title || '', artist: it.author || it.artist || '', genre, raw: it });
    }

    const genreList = Array.from(genres);
  const palette = ['#ff6b6b','#4ecdc4','#ffe66d','#a8e6cf','#ff8b94','#ffaaa5','#88d8b0','#ffd93d','#6c5ce7','#fdcb6e','#636e72','#00b894','#0984e3','#e84393','#00cec9'];
    const colorMap = {};
    genreList.forEach((g,i)=> colorMap[g]=palette[i%palette.length]);

    // Filter out hidden genres (read from window._hiddenGenres which is synced with React state)
    const hidden = window._hiddenGenres || new Set();
    const displayPts = pts.filter(p => !hidden.has(p.genre));
    
    const colors = displayPts.map(p => colorMap[p.genre] || '#888');
    const texts = displayPts.map(p => `${p.genre} — ${p.title} - ${p.artist}`);
    const ids = displayPts.map(p => p.id);

    const trace = {
      x: displayPts.map(p=>p.x),
      y: displayPts.map(p=>p.y),
      text: texts,
      customdata: ids,
      ids: ids,
      mode: 'markers',
      type: 'scattergl',
      // make markers slightly larger so they are visible on varied displays
      marker: { size: 6, opacity: 0.95, color: colors, line: { width: 0 } },
      name: 'tracks'
    };

    const layout = { 
      hovermode: 'closest', 
      dragmode: 'lasso', 
      legend: { orientation: 'h' }, 
      margin: { t: 20, b: 40, l: 40, r: 20 },
      paper_bgcolor: '#1f2937',
      plot_bgcolor: '#111827',
      font: { color: '#d1d5db' }
    };

    // export globals used by original handlers and update React state
    window._plotPointsFull = pts;
    window._plotPoints = displayPts;
    window._colorMap = colorMap;
    window._genreList = genreList;
    
    // Update React state with genres for UI rendering (use callback to access latest state)
    setGenres(genreList);

    const LARGE = 30000;
    // Diagnostic logging to help debug invisible plots
    try { console.debug('renderPlot: pts=', pts.length, 'first3=', pts.slice(0,3)); } catch(e){}
    try {
      if (pts.length > LARGE) {
        Plotly.purge(gd);
        Plotly.newPlot(gd, [trace], layout, {responsive:true}).then(()=>{
          attachPlotHandlers(gd);
        }).catch((err)=>{
          console.warn('Plotly.newPlot failed (scattergl), attempting fallback to scatter', err);
          // fallback to non-GL scatter
          try { trace.type = 'scatter'; Plotly.react(gd, [trace], layout, {responsive:true}).then(()=>attachPlotHandlers(gd)).catch(()=>attachPlotHandlers(gd)); } catch(e){ console.warn('Fallback plotting also failed', e); }
        });
      } else {
        Plotly.react(gd, [trace], layout, {responsive:true}).then(()=>{
          attachPlotHandlers(gd);
        }).catch((err)=>{
          console.warn('Plotly.react failed (scattergl), attempting fallback to scatter', err);
          try { trace.type = 'scatter'; Plotly.react(gd, [trace], layout, {responsive:true}).then(()=>attachPlotHandlers(gd)).catch(()=>attachPlotHandlers(gd)); } catch(e){ console.warn('Fallback plotting also failed', e); }
        });
      }
    } catch(e) { console.warn('renderPlot failed', e); }
  }, [attachPlotHandlers]);

  // load map data and render via Plotly
  useEffect(() => {
    let cancelled = false;
    const load = async () => {
      setLoading(true);
      setError('');
      try {
        const res = await apiFetch(`/api/map?percent=${percent}`);
        if (!res.ok) throw new Error(`Map fetch failed: ${res.status}`);
        const data = await res.json();
        if (cancelled) return;
        const items = data.items || [];
        // Store raw items for re-rendering when filtering genres
        rawItemsRef.current = items;
        // Small delay to ensure DOM is fully ready (race condition fix)
        setTimeout(() => {
          if (!cancelled) {
            console.log('About to call renderPlot with', items.length, 'items');
            renderPlot(items, data.projection);
          }
        }, 100);
      } catch (err) {
        console.error('Failed to load map data', err);
        setError('Failed to load map data');
      }
      setLoading(false);
    };
    fetchRef.current += 1;
    load();
    return () => { cancelled = true; };
  }, [percent, renderPlot]);

  // Autocomplete search - when user selects a song, add to selection and highlight on map
  useEffect(() => {
    const t = setTimeout(async () => {
      // Require at least 3 chars to reduce noise and match song view behavior
      if ((!query || query.length < 2)) { setSuggestions([]); return; }
      try {
        // Use the same search as the Songs view (subsonic search2.view) to avoid voyager proxy 404s
        const data = await searchMusic(query, { songCount: 50 });
        // searchMusic returns the Subsonic response object (searchResult2/searchResult3)
        const songs = (data.searchResult2 && data.searchResult2.song) || (data.searchResult3 && data.searchResult3.song) || [];
        const list = Array.isArray(songs) ? songs : (songs ? [songs] : []);
        // Normalize to an array of simple objects the Map UI expects
        const normalized = list.map(s => ({ item_id: s.id || s.itemId || '', title: s.title || s.name || '', author: s.artist || s.author || '' }));
        setSuggestions(normalized);
      } catch (e) { console.warn('voyager/subsonic search failed', e); setSuggestions([]); }
    }, 250);
    return () => clearTimeout(t);
  }, [query]);

  // Select and highlight a song on the map (from search results)
  const handleSelectSong = (songId) => {
    if (!songId) return;
    
    // Add to selection
    window._plotSelection = window._plotSelection || [];
    const sid = String(songId);
    if (!window._plotSelection.includes(sid)) {
      window._plotSelection.push(sid);
      setSelectedIds([...window._plotSelection]);
    }
    
    // Highlight on map with a yellow circle overlay
    const gd = plotDivRef.current || document.getElementById('map-plot');
    if (!gd) return;
    
    const pt = (window._plotPointsFull || []).find(p => String(p.id) === sid);
    if (!pt || pt.x === undefined || pt.y === undefined) {
      alert('Song not found on the current map view. Try a different percentage.');
      return;
    }
    
    // Calculate radius relative to axis range - BIGGER circle for better visibility
    let xRange = null, yRange = null;
    try {
      xRange = (gd.layout && gd.layout.xaxis && gd.layout.xaxis.range) ? gd.layout.xaxis.range : (gd._fullLayout && gd._fullLayout.xaxis && gd._fullLayout.xaxis.range ? gd._fullLayout.xaxis.range : null);
      yRange = (gd.layout && gd.layout.yaxis && gd.layout.yaxis.range) ? gd.layout.yaxis.range : (gd._fullLayout && gd._fullLayout.yaxis && gd._fullLayout.yaxis.range ? gd._fullLayout.yaxis.range : null);
    } catch (e) {}
    
    const rx = (xRange && xRange[1] - xRange[0]) ? (xRange[1] - xRange[0]) * 0.02 : 0.2;
    const ry = (yRange && yRange[1] - yRange[0]) ? (yRange[1] - yRange[0]) * 0.02 : 0.2;
    
    const highlightShape = {
      type: 'circle',
      xref: 'x',
      yref: 'y',
      x0: pt.x - rx,
      x1: pt.x + rx,
      y0: pt.y - ry,
      y1: pt.y + ry,
      fillcolor: 'rgba(255,235,59,0.35)',
      opacity: 0.95,
      line: { color: 'black', width: 3 },
      layer: 'above',
      am_type: 'search-highlight'
    };
    
    // Remove old highlights first
    try {
      const shapes = (gd.layout && Array.isArray(gd.layout.shapes)) ? gd.layout.shapes.slice() : [];
      const filtered = shapes.filter(s => !(s && s.am_type === 'search-highlight'));
      filtered.push(highlightShape);
      Plotly.relayout(gd, { shapes: filtered }).catch(() => {});
    } catch (e) {
      console.warn('Failed to add highlight', e);
    }
    
    // Clear search
    setQuery('');
    setSuggestions([]);
  };


  // Initialize selection on mount
  useEffect(() => {
    window._plotSelection = [];
    console.log('Map mounted: initialized window._plotSelection to empty array');
    
    // Clear the flag when Map unmounts
    return () => {
      window._mapAddedSong = false;
      console.log('Map unmounted: cleared _mapAddedSong flag');
    };
  }, []);

  // selection bridge used by UI controls
  useEffect(() => {
    const onSel = () => {
      const arr = window._plotSelection || [];
      console.log('am-map-selection-changed event received, window._plotSelection has', arr.length, 'items');
      const newSelection = Array.from(new Set(arr.map(String)));
      console.log('Updating selectedIds state to:', newSelection);
      setSelectedIds(newSelection);
    };
    window.addEventListener('am-map-selection-changed', onSel);
    return () => window.removeEventListener('am-map-selection-changed', onSel);
  }, []);

  // Play selected songs: fetch full song details, add to queue, start playing, navigate to songs tab
  const handlePlaySelection = async () => {
    const sel = window._plotSelection || selectedIds || [];
    if (!sel || sel.length === 0) return alert('No items selected');
    if (!onAddToQueue || !onPlay) return alert('Playback not available');
    
    try {
      // Fetch full song details for all selected IDs using Subsonic API
      const songPromises = sel.map(async (id) => {
        try {
          const data = await subsonicFetch('getSong.view', { id });
          return data.song || null;
        } catch (err) {
          console.warn('Failed to fetch song', id, err);
          return null;
        }
      });
      
      const songs = (await Promise.all(songPromises)).filter(Boolean);
      
      if (songs.length === 0) return alert('No valid songs found');
      
      // Add all songs to queue
      songs.forEach(song => onAddToQueue(song));
      
      // Start playing the first song
      onPlay(songs[0], songs);
      
      // Navigate to songs tab with the fetched songs as preloadedSongs so they display in the list
      if (onNavigate) {
        onNavigate({ page: 'songs', title: 'Playing from Map', filter: { preloadedSongs: songs } });
      }
      
      // Clear selection after playing
      window._plotSelection = [];
      setSelectedIds([]);
    } catch (err) {
      console.error('Failed to play selection', err);
      alert('Failed to load songs for playback');
    }
  };

  // Create Path: compute paths between consecutive pairs of selected songs and plot on map
  const handleCreatePath = async () => {
    const sel = (window._plotSelection || selectedIds || []).slice(0, 10); // cap at 10
    if (!sel || sel.length < 2) return alert('Select at least 2 songs (max 10) to create a path');
    
    try {
      const allPathSongs = [];
      
      // For each consecutive pair, call Subsonic getSongPath.view API
      for (let i = 0; i < sel.length - 1; i++) {
        const startId = sel[i];
        const endId = sel[i + 1];
        
        try {
          // Use Subsonic API getSongPath.view (same as Up Next list button)
          const data = await subsonicFetch('getSongPath.view', { startId, endId });
          
          // Subsonic returns { directory: { song: [...] } }
          const pathSongs = data.directory?.song || [];
          const songList = Array.isArray(pathSongs) ? pathSongs : (pathSongs ? [pathSongs] : []);
          
          console.log('Path songs received for', startId, '->', endId, ':', songList.length, 'songs');
          
          if (songList.length > 0) {
            allPathSongs.push(...songList);
          }
        } catch (err) {
          console.warn('Failed to compute path for pair', startId, endId, err);
        }
      }
      
      if (allPathSongs.length === 0) return alert('No path found');
      
      // Add all path songs to selection AND queue
      allPathSongs.forEach(s => {
        const sid = String(s.id || '');
        if (sid && !window._plotSelection.includes(sid)) {
          window._plotSelection.push(sid);
          // Add to queue
          if (onAddToQueue) {
            onAddToQueue(s);
          }
        }
      });
      
      // Set flag so AudioPlayer won't auto-play
      window._mapAddedSong = true;
      
      // Safety: clear flag after a short delay
      setTimeout(() => {
        if (window._mapAddedSong) {
          window._mapAddedSong = false;
          console.log('Map: Auto-cleared _mapAddedSong flag after path creation');
        }
      }, 1000);
      
      // Update React state (will trigger re-render with correct count)
      setSelectedIds([...window._plotSelection]);
      
      // Dispatch selection changed event
      try {
        window.dispatchEvent(new CustomEvent('am-map-selection-changed'));
      } catch (e) {
        console.warn('Failed to dispatch selection event', e);
      }
      
      // Draw path on map - normalize song objects to have item_id field
      const normalizedSongs = allPathSongs.map(s => ({
        ...s,
        item_id: s.id,
        author: s.artist
      }));
      
      // Check how many songs are actually on the current map
      const plotted = window._plotPointsFull || [];
      const plottedIds = new Set(plotted.map(p => String(p.id)));
      const visibleCount = normalizedSongs.filter(s => plottedIds.has(String(s.id))).length;
      
      console.log('Attempting to draw path with', allPathSongs.length, 'songs,', visibleCount, 'visible on map');
      
      drawPathOnMap(normalizedSongs);
      
      // Only show alert if there's an error (no visible songs)
      if (visibleCount === 0) {
        alert(`Path received ${allPathSongs.length} songs but NONE are visible on the current map view. Try selecting 100% to see all songs.`);
      }
    } catch (err) {
      console.error('Failed to create path', err);
      alert('Failed to create path');
    }
  };

  // Refresh map: clear everything and reload
  const handleRefresh = () => {
    try {
      const gd = plotDivRef.current || document.getElementById('map-plot');
      
      // Remove all shape overlays (paths, highlights)
      if (gd) {
        Plotly.relayout(gd, { shapes: [] }).catch(() => {});
      }
      
      // Clear the entire play queue using the Dashboard's clearQueue function
      if (onClearQueue) {
        onClearQueue();
      }
      
      // Clear selection (React state update will handle UI)
      window._plotSelection = [];
      setSelectedIds([]);
      
      // Clear all genre filters
      setHiddenGenres(new Set());
      window._hiddenGenres = new Set();
      
      // Reset overlay visibility toggles
      setShowSearchHighlight(true);
      setShowPathLine(true);
      setShowPathPoints(true);
      
      // Reload the map with current percentage
      const items = rawItemsRef.current;
      if (items && items.length > 0) {
        renderPlot(items, null);
      }
      
      console.log('Map refreshed: cleared everything (including queue) and reloaded');
    } catch (e) {
      console.warn('Failed to refresh map', e);
    }
  };

  // Genre filtering functions - preserve zoom/axis ranges
  const toggleGenre = (genre) => {
    const newHidden = new Set(hiddenGenres);
    if (newHidden.has(genre)) {
      newHidden.delete(genre);
    } else {
      newHidden.add(genre);
    }
    setHiddenGenres(newHidden);
    window._hiddenGenres = newHidden;
    
    // Capture current axis ranges before re-rendering
    const gd = plotDivRef.current || document.getElementById('map-plot');
    let preservedShapes = [];
    let xRange = null, yRange = null;
    if (gd) {
      try {
        if (gd.layout && Array.isArray(gd.layout.shapes)) {
          preservedShapes = gd.layout.shapes.slice();
        }
        xRange = (gd.layout && gd.layout.xaxis && gd.layout.xaxis.range) ? gd.layout.xaxis.range.slice() : null;
        yRange = (gd.layout && gd.layout.yaxis && gd.layout.yaxis.range) ? gd.layout.yaxis.range.slice() : null;
      } catch (e) {}
    }
    
    // Re-render the plot with updated filters
    const items = rawItemsRef.current;
    if (items && items.length > 0) {
      renderPlot(items, null);
      
      // Restore axis ranges and shapes after render
      setTimeout(() => {
        if (gd && (xRange || yRange)) {
          const update = {};
          if (xRange) update['xaxis.range'] = xRange;
          if (yRange) update['yaxis.range'] = yRange;
          Plotly.relayout(gd, update).catch(() => {});
        }
        if (gd && preservedShapes.length > 0) {
          Plotly.relayout(gd, { shapes: preservedShapes }).catch(() => {});
        }
      }, 50);
    }
  };
  
  const hideAllGenres = () => {
    const newHidden = new Set(genres);
    setHiddenGenres(newHidden);
    window._hiddenGenres = newHidden;
    
    // Capture current axis ranges
    const gd = plotDivRef.current || document.getElementById('map-plot');
    let preservedShapes = [];
    let xRange = null, yRange = null;
    if (gd) {
      try {
        if (gd.layout && Array.isArray(gd.layout.shapes)) {
          preservedShapes = gd.layout.shapes.slice();
        }
        xRange = (gd.layout && gd.layout.xaxis && gd.layout.xaxis.range) ? gd.layout.xaxis.range.slice() : null;
        yRange = (gd.layout && gd.layout.yaxis && gd.layout.yaxis.range) ? gd.layout.yaxis.range.slice() : null;
      } catch (e) {}
    }
    
    const items = rawItemsRef.current;
    if (items && items.length > 0) {
      renderPlot(items, null);
      
      setTimeout(() => {
        if (gd && (xRange || yRange)) {
          const update = {};
          if (xRange) update['xaxis.range'] = xRange;
          if (yRange) update['yaxis.range'] = yRange;
          Plotly.relayout(gd, update).catch(() => {});
        }
        if (gd && preservedShapes.length > 0) {
          Plotly.relayout(gd, { shapes: preservedShapes }).catch(() => {});
        }
      }, 50);
    }
  };
  
  const showAllGenres = () => {
    setHiddenGenres(new Set());
    window._hiddenGenres = new Set();
    
    // Capture current axis ranges
    const gd = plotDivRef.current || document.getElementById('map-plot');
    let preservedShapes = [];
    let xRange = null, yRange = null;
    if (gd) {
      try {
        if (gd.layout && Array.isArray(gd.layout.shapes)) {
          preservedShapes = gd.layout.shapes.slice();
        }
        xRange = (gd.layout && gd.layout.xaxis && gd.layout.xaxis.range) ? gd.layout.xaxis.range.slice() : null;
        yRange = (gd.layout && gd.layout.yaxis && gd.layout.yaxis.range) ? gd.layout.yaxis.range.slice() : null;
      } catch (e) {}
    }
    
    const items = rawItemsRef.current;
    if (items && items.length > 0) {
      renderPlot(items, null);
      
      setTimeout(() => {
        if (gd && (xRange || yRange)) {
          const update = {};
          if (xRange) update['xaxis.range'] = xRange;
          if (yRange) update['yaxis.range'] = yRange;
          Plotly.relayout(gd, update).catch(() => {});
        }
        if (gd && preservedShapes.length > 0) {
          Plotly.relayout(gd, { shapes: preservedShapes }).catch(() => {});
        }
      }, 50);
    }
  };

  // Draw path polyline and points on the map (like HTML version)
  const drawPathOnMap = (pathItems) => {
    if (!pathItems || pathItems.length === 0) {
      console.warn('drawPathOnMap: no path items');
      return;
    }
    
    const gd = plotDivRef.current || document.getElementById('map-plot');
    if (!gd) {
      console.warn('drawPathOnMap: plot element not found');
      return;
    }
    
    console.log('drawPathOnMap: processing', pathItems.length, 'items');
    
    // Build coordinate arrays by looking up each song in the plotted points
    const xs = [];
    const ys = [];
    const colorMap = window._colorMap || {};
    const plotted = window._plotPointsFull || [];
    
    console.log('Available plotted points:', plotted.length);
    
    for (const it of pathItems) {
      let x, y;
      const songId = String(it.item_id || it.id || '');
      
      // First check if song has embedding_2d coordinates
      if (it.embedding_2d && Array.isArray(it.embedding_2d) && it.embedding_2d.length >= 2) {
        x = it.embedding_2d[0];
        y = it.embedding_2d[1];
        console.log('Found coords from embedding_2d for song', songId, ':', x, y);
      } else {
        // Fallback: search in plotted points by ID
        const pt = plotted.find(p => String(p.id) === songId);
        if (pt && pt.x !== undefined && pt.y !== undefined) {
          x = pt.x;
          y = pt.y;
          console.log('Found coords from plotted points for song', songId, ':', x, y);
        } else {
          console.warn('No coordinates found for song', songId, it.title || '(no title)');
        }
      }
      
      if (x !== undefined && y !== undefined) {
        xs.push(x);
        ys.push(y);
      }
    }
    
    console.log('Final coordinate arrays:', xs.length, 'points');
    
    if (xs.length === 0) {
      console.warn('drawPathOnMap: no valid coordinates found');
      return;
    }
    
    // Calculate radius for point markers
    let xRange = null, yRange = null;
    try {
      xRange = (gd.layout && gd.layout.xaxis && gd.layout.xaxis.range) ? gd.layout.xaxis.range : (gd._fullLayout && gd._fullLayout.xaxis && gd._fullLayout.xaxis.range ? gd._fullLayout.xaxis.range : null);
      yRange = (gd.layout && gd.layout.yaxis && gd.layout.yaxis.range) ? gd.layout.yaxis.range : (gd._fullLayout && gd._fullLayout.yaxis && gd._fullLayout.yaxis.range ? gd._fullLayout.yaxis.range : null);
    } catch (e) {}
    
    const rx = (xRange && xRange[1] - xRange[0]) ? (xRange[1] - xRange[0]) * 0.0075 : 0.15;
    const ry = (yRange && yRange[1] - yRange[0]) ? (yRange[1] - yRange[0]) * 0.0075 : 0.15;
    
    // Build SVG path string
    let pathStr = '';
    for (let i = 0; i < xs.length; i++) {
      pathStr += (i === 0 ? `M ${xs[i]} ${ys[i]}` : ` L ${xs[i]} ${ys[i]}`);
    }
    
    const pathShape = {
      type: 'path',
      path: pathStr,
      xref: 'x',
      yref: 'y',
      line: { color: 'black', width: 3 },
      layer: 'above',
      am_type: 'path-line'
    };
    
    // Create circle markers for each point
    const pointShapes = xs.map((x, i) => ({
      type: 'circle',
      xref: 'x',
      yref: 'y',
      x0: x - rx,
      x1: x + rx,
      y0: ys[i] - ry,
      y1: ys[i] + ry,
      fillcolor: colorMap['path'] || '#ff6b6b',
      opacity: 0.95,
      line: { color: 'black', width: 1.5 },
      layer: 'above',
      am_type: 'path-point'
    }));
    
    // Add shapes to plot
    try {
      const existing = (gd.layout && Array.isArray(gd.layout.shapes)) ? gd.layout.shapes.slice() : [];
      console.log('Adding path shapes: 1 line +', pointShapes.length, 'points. Existing shapes:', existing.length);
      const merged = existing.concat([pathShape, ...pointShapes]);
      Plotly.relayout(gd, { shapes: merged }).then(() => {
        console.log('Path drawn successfully!');
      }).catch((err) => {
        console.warn('Plotly.relayout failed', err);
      });
    } catch (e) {
      console.warn('Failed to draw path', e);
    }
  };

  // Toggle overlay visibility by filtering shapes
  const toggleOverlay = (amType, show) => {
    const gd = plotDivRef.current || document.getElementById('map-plot');
    if (!gd) return;
    
    try {
      const shapes = (gd.layout && Array.isArray(gd.layout.shapes)) ? gd.layout.shapes.slice() : [];
      const filtered = shapes.map(shape => {
        if (shape && shape.am_type === amType) {
          return { ...shape, visible: show };
        }
        return shape;
      });
      Plotly.relayout(gd, { shapes: filtered }).catch(() => {});
    } catch (e) {
      console.warn('Failed to toggle overlay', e);
    }
  };

  return (
    <div className="text-gray-100">
      <div className="mb-2 flex gap-1.5 sm:gap-2 items-center flex-wrap">
        <input value={query} onChange={e => setQuery(e.target.value)} placeholder="Search..." className="bg-gray-800 border border-gray-700 text-gray-100 rounded px-2 py-1 text-sm w-32 sm:w-48 md:w-64" />
        <div className="flex gap-2 items-center">
          <label className="text-gray-400 text-xs sm:text-sm flex items-center">Size:</label>
          <select value={percent} onChange={e => setPercent(Number(e.target.value))} className="bg-gray-800 border border-gray-700 text-gray-100 rounded px-2 py-1 text-sm">
            <option value={10}>10%</option>
            <option value={25}>25%</option>
            <option value={50}>50%</option>
            <option value={100}>100%</option>
          </select>
        </div>
        <div id="map-status" className="text-gray-300 text-xs sm:text-sm ml-1 sm:ml-4">Selected: {selectedIds.length}</div>
        <button onClick={handleRefresh} className="bg-gray-700 hover:bg-gray-600 px-2 sm:px-4 py-1 rounded text-white text-xs sm:text-sm" title="Clear overlays and selection">
          🔄 <span className="hidden sm:inline">Refresh</span>
        </button>
        {selectedIds.length >= 2 && (
          <button 
            onClick={selectedIds.length <= 10 ? handleCreatePath : undefined} 
            disabled={selectedIds.length > 10}
            className={selectedIds.length <= 10 
              ? "bg-yellow-600 hover:bg-yellow-700 px-2 sm:px-4 py-1 rounded text-white font-semibold cursor-pointer text-xs sm:text-sm" 
              : "bg-gray-600 px-2 sm:px-4 py-1 rounded text-gray-400 font-semibold cursor-not-allowed opacity-50 text-xs sm:text-sm"
            }
            title={selectedIds.length > 10 ? "Maximum 10 songs allowed for path creation" : "Create path between selected songs"}
          >
            🛤️ <span className="hidden sm:inline">Path</span> ({selectedIds.length})
          </button>
        )}
        {selectedIds.length > 0 && (
          <button onClick={handlePlaySelection} className="bg-green-600 hover:bg-green-700 px-2 sm:px-4 py-1 rounded text-white font-semibold text-xs sm:text-sm">
            ▶ <span className="hidden sm:inline">Play</span> ({selectedIds.length})
          </button>
        )}
      </div>

      {suggestions.length > 0 && (
        <div className="mb-2 bg-gray-800 border border-gray-700 rounded p-2 text-sm max-w-md">
          {suggestions.map((s, i) => (
            <div key={i} className="py-1 cursor-pointer hover:bg-gray-700" onClick={() => handleSelectSong(s.item_id || s.id)}>{s.title || s.name} — <span className="text-gray-400">{s.author || s.artist}</span></div>
          ))}
        </div>
      )}

      {loading && <div className="text-gray-300">Loading map...</div>}
      {error && <div className="text-red-400">{error}</div>}

      {!loading && !error && (
        <div>
          <div id="map-plot" ref={plotDivRef} style={{ width: '100%', height: '400px', minHeight: '400px', backgroundColor: '#1f2937', border: '1px solid #374151' }} className="sm:!h-[500px] md:!h-[600px]" />
          
          {/* Interactive genre legend with show/hide controls */}
          <div className="mt-3 p-2 sm:p-3 bg-gray-800 border border-gray-700 rounded">
            <div className="flex items-center gap-1.5 sm:gap-4 mb-2 flex-wrap">
              <span className="font-semibold text-gray-300 text-xs sm:text-sm">Genres:</span>
              <button onClick={showAllGenres} className="text-xs px-1.5 sm:px-2 py-1 bg-gray-700 hover:bg-gray-600 rounded text-gray-300">
                Show All
              </button>
              <button onClick={hideAllGenres} className="text-xs px-1.5 sm:px-2 py-1 bg-gray-700 hover:bg-gray-600 rounded text-gray-300">
                Hide All
              </button>
              
              {/* Overlay toggles */}
              <span className="text-gray-500 mx-1 sm:mx-2">|</span>
              <span className="text-gray-400 text-xs">Overlays:</span>
              <button 
                onClick={() => {
                  setShowSearchHighlight(!showSearchHighlight);
                  toggleOverlay('search-highlight', !showSearchHighlight);
                }}
                className={`text-xs px-2 py-1 rounded ${showSearchHighlight ? 'bg-yellow-600 text-white' : 'bg-gray-700 text-gray-400'}`}
                title={showSearchHighlight ? 'Hide search highlight' : 'Show search highlight'}
              >
                {showSearchHighlight ? '👁' : '👁‍🗨'} Search
              </button>
              <button 
                onClick={() => {
                  setShowPathLine(!showPathLine);
                  toggleOverlay('path-line', !showPathLine);
                }}
                className={`text-xs px-2 py-1 rounded ${showPathLine ? 'bg-blue-600 text-white' : 'bg-gray-700 text-gray-400'}`}
                title={showPathLine ? 'Hide path line' : 'Show path line'}
              >
                {showPathLine ? '👁' : '👁‍🗨'} Path Line
              </button>
              <button 
                onClick={() => {
                  setShowPathPoints(!showPathPoints);
                  toggleOverlay('path-point', !showPathPoints);
                }}
                className={`text-xs px-2 py-1 rounded ${showPathPoints ? 'bg-red-600 text-white' : 'bg-gray-700 text-gray-400'}`}
                title={showPathPoints ? 'Hide path points' : 'Show path points'}
              >
                {showPathPoints ? '👁' : '👁‍🗨'} Path Points
              </button>
            </div>
            <div className="flex flex-wrap gap-2">
              {genres.slice(0, 50).map(genre => {
                const colorMap = window._colorMap || {};
                const color = colorMap[genre] || '#888';
                const isHidden = hiddenGenres.has(genre);
                return (
                  <button
                    key={genre}
                    onClick={() => toggleGenre(genre)}
                    className={`flex items-center gap-1 px-2 py-1 rounded text-xs ${isHidden ? 'opacity-40 line-through' : ''} hover:bg-gray-700 transition-opacity`}
                    style={{ cursor: 'pointer' }}
                  >
                    <span 
                      className="inline-block w-3 h-3 rounded-sm" 
                      style={{ backgroundColor: color }}
                    />
                    <span className="text-gray-300">{genre}</span>
                  </button>
                );
              })}
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
