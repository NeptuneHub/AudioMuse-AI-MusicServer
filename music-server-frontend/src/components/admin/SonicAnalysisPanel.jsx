import React, { useState, useEffect, useCallback } from 'react';
import { API_BASE } from '../../api';

function SonicAnalysisPanel() {
    const [status, setStatus] = useState(null);
    const [error, setError] = useState('');
    const [isLoading, setIsLoading] = useState(true);
    const [isStarting, setIsStarting] = useState(false);
    const [audioMuseConfigured, setAudioMuseConfigured] = useState(null); // null = checking, true = configured, false = not configured

    // Check if AudioMuse-AI Core URL is configured
    const checkAudioMuseConfiguration = useCallback(async () => {
        const token = localStorage.getItem('token');
        try {
            const response = await fetch(`${API_BASE}/rest/getConfiguration.view?f=json`, {
                headers: { 'Authorization': `Bearer ${token}` }
            });
            if (!response.ok) {
                throw new Error(`Failed to check configuration: ${response.status}`);
            }
            const data = await response.json();
            const config = data['subsonic-response']?.configurations?.configuration;
            const audioMuseUrl = config?.find(c => c.name === 'audiomuse_ai_core_url')?.value;
            const isConfigured = !!(audioMuseUrl && audioMuseUrl.trim());
            setAudioMuseConfigured(isConfigured);
            return isConfigured;
        } catch (err) {
            console.error("Failed to check AudioMuse configuration:", err);
            setAudioMuseConfigured(false);
            return false;
        }
    }, []);

    const fetchStatus = useCallback(async () => {
        const token = localStorage.getItem('token');
        try {
            // Use the Subsonic endpoint with absolute URL for production
            const response = await fetch(`${API_BASE}/rest/getSonicAnalysisStatus.view?f=json`, {
                headers: { 'Authorization': `Bearer ${token}` }
            });
            if (!response.ok) {
                // If it's 503 Service Unavailable, it likely means AudioMuse-AI is not configured
                if (response.status === 503) {
                    throw new Error("AudioMuse-AI Core URL not configured");
                }
                const errData = await response.json().catch(() => ({ error: `Server error: ${response.status}` }));        
                throw new Error(errData.error || `Server error: ${response.status}`);
            }
            const data = await response.json();
            console.log('DEBUG: Status response from getSonicAnalysisStatus:', data);
            setStatus(data);
            setError('');
        } catch (err) {
            if (err.message.includes("AudioMuse-AI Core URL not configured")) {
                setError("AudioMuse-AI Core URL not configured. Please configure it in the admin panel to use this feature.");
                setStatus(null);
                setIsLoading(false);
                return; // Don't continue polling if not configured
            }
            setError(err.message);
            console.error("Failed to fetch analysis status:", err);
        } finally {
            setIsLoading(false);
        }
    }, []);

    useEffect(() => {
        const initializePanel = async () => {
            const isConfigured = await checkAudioMuseConfiguration();
            if (isConfigured) {
                fetchStatus();
            } else {
                setError("AudioMuse-AI Core URL not configured. Please configure it in the admin panel to use this feature.");
                setIsLoading(false);
            }
        };
        
        initializePanel();
        
        // Only set up polling if AudioMuse-AI is configured
        const intervalId = setInterval(async () => {
            if (audioMuseConfigured === null) {
                // Still checking configuration, skip this interval
                return;
            }
            if (audioMuseConfigured && (!error || !error.includes("AudioMuse-AI Core URL not configured"))) {
                fetchStatus();
            }
        }, 5000);
        
        return () => clearInterval(intervalId);
    }, [fetchStatus, checkAudioMuseConfiguration, audioMuseConfigured, error]);

    const startTask = async (endpoint, taskName) => {
        setError('');
        setIsStarting(true);
        const token = localStorage.getItem('token');
        try {
            // Use the Subsonic endpoint with absolute URL and original working format
            const response = await fetch(`${API_BASE}${endpoint}?f=json`, {
                method: 'POST',
                headers: {
                    'Authorization': `Bearer ${token}`,
                    'Content-Type': 'application/json'
                },
                body: JSON.stringify({})
            });
            if (!response.ok) {
                const errData = await response.json().catch(() => ({ error: `Failed to start ${taskName}` }));
                throw new Error(errData.error || `Failed to start ${taskName}`);
            }
            // Optimistically update status for immediate UI feedback
            setStatus(prev => ({ ...(prev || {}), status: 'PENDING', details: { ...(prev?.details || {}), status_message: `${taskName} initiated.` } }));
            await fetchStatus();
        } catch (err) {
            setError(err.message);
        } finally {
            setIsStarting(false);
        }
    };

    // Point to the Subsonic endpoints
    const handleStartClustering = () => startTask('/rest/startSonicClustering.view', 'Clustering');
    const handleStart = () => startTask('/rest/startSonicAnalysis.view', 'Analysis');

    // Start cleaning - lightweight endpoint outside Subsonic REST namespace
    const handleStartCleaning = async () => {
        setError('');
        setIsStarting(true);
        const token = localStorage.getItem('token');
        try {
            const response = await fetch(`${API_BASE}/api/cleaning/start`, {
                method: 'POST',
                headers: {
                    'Authorization': `Bearer ${token}`,
                    'Content-Type': 'application/json'
                },
                body: JSON.stringify({})
            });
            if (!response.ok) {
                const errData = await response.json().catch(() => ({ error: 'Failed to start cleaning' }));
                throw new Error(errData.error || 'Failed to start cleaning');
            }
            // Give immediate feedback and refresh status/logs
            setStatus(prev => ({ ...(prev || {}), status: 'PENDING', details: { ...(prev?.details || {}), status_message: 'Cleaning initiated.' } }));
            await fetchStatus();
        } catch (err) {
            setError(err.message);
        } finally {
            setIsStarting(false);
        }
    };

    const handleCancel = async () => {
        if (!status || !status.task_id) {
            setError("No active task to cancel.");
            return;
        }
        setError('');
        const token = localStorage.getItem('token');
        try {
            // Use the Subsonic endpoint with original working format
            const response = await fetch(`${API_BASE}/rest/cancelSonicAnalysis.view?f=json&taskId=${status.task_id}`, {
                method: 'POST',
                headers: { 'Authorization': `Bearer ${token}` }
            });
            if (!response.ok) {
                 const errData = await response.json().catch(() => ({ error: 'Failed to cancel analysis' }));
                throw new Error(errData.error || 'Failed to cancel analysis');
            }
            await fetchStatus();
        } catch (err) {
            setError(err.message);
        }
    };

    const isTaskRunning = status && (status.status === 'PROGRESS' || status.status === 'STARTED' || status.status === 'PENDING');
    const isAudioMuseConfigured = audioMuseConfigured === true;

    return (
        <div className="bg-gray-800 p-4 sm:p-6 rounded-lg">
            <h3 className="text-xl font-bold mb-4">Analysis and Clustering</h3>
            {error && <p className="text-red-500 mb-4 p-3 bg-red-900/50 rounded">{error}</p>}
            
            {!isAudioMuseConfigured && audioMuseConfigured !== null && (
                <div className="mb-4 p-4 bg-yellow-900/50 border border-yellow-600 rounded">
                    <p className="text-yellow-300 mb-2">
                        AudioMuse-AI Core URL is not configured. This feature requires the AudioMuse-AI container to be running and configured.
                    </p>
                    <button
                        onClick={async () => {
                            setIsLoading(true);
                            const isConfigured = await checkAudioMuseConfiguration();
                            if (isConfigured) {
                                fetchStatus();
                            } else {
                                setIsLoading(false);
                            }
                        }}
                        className="bg-yellow-600 hover:bg-yellow-700 text-white font-bold py-2 px-4 rounded"
                    >
                        Refresh Configuration
                    </button>
                </div>
            )}
            
            <div className="flex justify-end mb-4">
                <button
                    onClick={handleStartClustering}
                    disabled={!isAudioMuseConfigured || isTaskRunning || isLoading || isStarting}
                    className="bg-blue-600 hover:bg-blue-700 text-white font-bold py-2 px-4 rounded disabled:bg-gray-500 disabled:cursor-not-allowed mr-4"
                >
                    {isStarting ? 'Starting...' : 'Start Clustering'}
                </button>
                <button
                    onClick={handleStart}
                    disabled={!isAudioMuseConfigured || isTaskRunning || isLoading || isStarting}
                    className="bg-green-600 hover:bg-green-700 text-white font-bold py-2 px-4 rounded disabled:bg-gray-500 disabled:cursor-not-allowed mr-4"
                >
                    {isStarting ? 'Starting...' : (isLoading && !isTaskRunning ? 'Loading...' : 'Start New Analysis')}
                </button>

                {/* Cleaning task uses backend cleaning API (not part of AudioMuse-AI) */}
                <button
                    onClick={handleStartCleaning}
                    disabled={isTaskRunning || isLoading || isStarting}
                    className="bg-indigo-600 hover:bg-indigo-700 text-white font-bold py-2 px-4 rounded disabled:bg-gray-500 disabled:cursor-not-allowed"
                    title="Start database cleaning (will call /api/cleaning/start)"
                >
                    {isStarting ? 'Starting...' : 'Start Cleaning'}
                </button>
                {isTaskRunning && (
                     <button
                        onClick={handleCancel}
                        disabled={isLoading}
                        className="ml-4 bg-red-600 hover:bg-red-700 text-white font-bold py-2 px-4 rounded disabled:bg-gray-500 disabled:cursor-not-allowed"
                    >
                        Cancel Task
                    </button>
                )}
            </div>

            {isLoading && !status && <p>Loading status...</p>}
            
            {status && (
                <div className="space-y-3">
                    <div>
                        <span className="font-semibold text-gray-400">Status: </span>
                        <span className={`font-mono px-2 py-1 rounded text-sm ${isTaskRunning ? 'bg-blue-900 text-blue-300' : (status.status === 'SUCCESS' ? 'bg-green-900 text-green-300' : 'bg-gray-700 text-gray-300')}`}>{status.status || 'N/A'}</span>
                    </div>
                    {isTaskRunning && (
                        <div>
                             <div className="w-full bg-gray-700 rounded-full h-2.5">
                                <div className="bg-teal-500 h-2.5 rounded-full" style={{ width: `${status.progress || 0}%` }}></div>
                            </div>
                            <p className="text-center text-sm mt-1">{Math.round(status.progress || 0)}%</p>
                        </div>
                    )}
                    <div>
                        <span className="font-semibold text-gray-400">Message: </span>
                        <span>{status.details?.status_message || 'No status message.'}</span>
                    </div>
                     <div>
                         <span className="font-semibold text-gray-400">Running Time: </span>
                        <span>{Math.round(status.running_time_seconds || 0)} seconds</span>
                    </div>
                    <div className="pt-2">
                        <h4 className="font-semibold text-gray-400 mb-1">Logs:</h4>
                        <div className="bg-gray-900 p-3 rounded-md h-40 overflow-y-auto font-mono text-xs text-gray-300">
                           {(() => {
                               // Debug what's actually in the status object
                               console.log('DEBUG: Full status object for logs:', status);
                               console.log('DEBUG: status.details:', status.details);
                               console.log('DEBUG: status.logs:', status.logs);
                               console.log('DEBUG: status.log:', status.log);
                               
                               // Try multiple possible log locations
                               const logs = status.details?.log || status.logs || status.log || status.details?.logs;
                               
                               if (logs && Array.isArray(logs) && logs.length > 0) {
                                   return logs.map((line, index) => (
                                       <p key={index}>{line}</p>
                                   ));
                               } else if (logs && typeof logs === 'string') {
                                   return <p>{logs}</p>;
                               } else {
                                   return <p>No logs available. Status: {status.status}, Task: {status.task_type || 'unknown'}</p>;
                               }
                           })()}
                        </div>
                    </div>
                </div>
            )}
        </div>
    );
}

export default SonicAnalysisPanel;
