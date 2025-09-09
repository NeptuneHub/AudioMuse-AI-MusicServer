import React, { useState, useEffect, useCallback } from 'react';

function SonicAnalysisPanel() {
    const [status, setStatus] = useState(null);
    const [error, setError] = useState('');
    const [isLoading, setIsLoading] = useState(true);
    const [isStarting, setIsStarting] = useState(false);

    const fetchStatus = useCallback(async () => {
        const token = localStorage.getItem('token');
        try {
            // Use the new Subsonic endpoint
            const response = await fetch('/rest/getSonicAnalysisStatus.view?f=json', {
                headers: { 'Authorization': `Bearer ${token}` }
            });
            if (!response.ok) {
                const errData = await response.json().catch(() => ({ error: `Server error: ${response.status}` }));
                throw new Error(errData.error || `Server error: ${response.status}`);
            }
            const data = await response.json();
            setStatus(data);
            setError('');
        } catch (err) {
            setError(err.message);
            console.error("Failed to fetch analysis status:", err);
        } finally {
            setIsLoading(false);
        }
    }, []);

    useEffect(() => {
        fetchStatus();
        const intervalId = setInterval(fetchStatus, 5000); // Poll every 5 seconds
        return () => clearInterval(intervalId);
    }, [fetchStatus]);

    const startTask = async (endpoint, taskName) => {
        setError('');
        setIsStarting(true);
        const token = localStorage.getItem('token');
        try {
            // Use the new Subsonic endpoint and add f=json
            const response = await fetch(`${endpoint}?f=json`, {
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
    
    // Point to the new Subsonic endpoints
    const handleStartClustering = () => startTask('/rest/startSonicClustering.view', 'Clustering');
    const handleStart = () => startTask('/rest/startSonicAnalysis.view', 'Analysis');

    const handleCancel = async () => {
        if (!status || !status.task_id) {
            setError("No active task to cancel.");
            return;
        }
        setError('');
        const token = localStorage.getItem('token');
        try {
            // Use the new Subsonic endpoint with taskId as a query parameter
            const response = await fetch(`/rest/cancelSonicAnalysis.view?f=json&taskId=${status.task_id}`, {
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

    return (
        <div className="bg-gray-800 p-4 sm:p-6 rounded-lg">
            <h3 className="text-xl font-bold mb-4">Analysis and Clustering</h3>
            {error && <p className="text-red-500 mb-4 p-3 bg-red-900/50 rounded">{error}</p>}
            
            <div className="flex justify-end mb-4">
                <button
                    onClick={handleStartClustering}
                    disabled={isTaskRunning || isLoading || isStarting}
                    className="bg-blue-600 hover:bg-blue-700 text-white font-bold py-2 px-4 rounded disabled:bg-gray-500 disabled:cursor-not-allowed mr-4"
                >
                    {isStarting ? 'Starting...' : 'Start Clustering'}
                </button>
                <button
                    onClick={handleStart}
                    disabled={isTaskRunning || isLoading || isStarting}
                    className="bg-green-600 hover:bg-green-700 text-white font-bold py-2 px-4 rounded disabled:bg-gray-500 disabled:cursor-not-allowed"
                >
                    {isStarting ? 'Starting...' : (isLoading && !isTaskRunning ? 'Loading...' : 'Start New Analysis')}
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
                           {(status.details?.log && status.details.log.length > 0) ? status.details.log.map((line, index) => (
                               <p key={index}>{line}</p>
                           )) : <p>No logs yet.</p>}
                        </div>
                    </div>
                </div>
            )}
        </div>
    );
}

export default SonicAnalysisPanel;
