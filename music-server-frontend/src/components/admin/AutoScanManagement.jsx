import React, { useState, useEffect, useCallback } from 'react';
import { subsonicFetch } from '../../api';

function AutoScanManagement({ onConfigChange }) {
    const [schedule, setSchedule] = useState('');
    const [isEnabled, setIsEnabled] = useState(false);
    // New schedules for Analysis and Clustering
    const [analysisSchedule, setAnalysisSchedule] = useState('');
    const [analysisEnabled, setAnalysisEnabled] = useState(false);
    const [clusteringSchedule, setClusteringSchedule] = useState('');
    const [clusteringEnabled, setClusteringEnabled] = useState(false);
    const [message, setMessage] = useState('');
    const [error, setError] = useState('');

    const subsonicApiRequest = useCallback(async (endpoint, params = {}) => {
        return await subsonicFetch(endpoint, params);
    }, []);

    const fetchConfig = useCallback(async () => {
        try {
            const data = await subsonicApiRequest('getConfiguration.view');
            const configList = data?.configurations?.configuration || [];
            const allConfigs = Array.isArray(configList) ? configList : [configList].filter(Boolean);
            
            const scheduleConfig = allConfigs.find(c => c.name === 'scan_schedule');
            setSchedule(scheduleConfig?.value || '0 2 * * *');

            const enabledConfig = allConfigs.find(c => c.name === 'scan_enabled');
            setIsEnabled(enabledConfig?.value === 'true');

            // Analysis schedule - default: every night at 2:00 except Saturday (Sun-Fri)
            const analysisConfig = allConfigs.find(c => c.name === 'analysis_schedule');
            setAnalysisSchedule(analysisConfig?.value || '0 2 * * 0-5');
            const analysisEnabledConfig = allConfigs.find(c => c.name === 'analysis_enabled');
            setAnalysisEnabled(analysisEnabledConfig?.value === 'true');

            // Clustering schedule - default: Saturday at 2:00
            const clusteringConfig = allConfigs.find(c => c.name === 'clustering_schedule');
            setClusteringSchedule(clusteringConfig?.value || '0 2 * * 6');
            const clusteringEnabledConfig = allConfigs.find(c => c.name === 'clustering_enabled');
            setClusteringEnabled(clusteringEnabledConfig?.value === 'true');

        } catch (err) {
            setError(err.message || 'Failed to fetch scan configuration');
        }
    }, [subsonicApiRequest]);

    useEffect(() => {
        fetchConfig();
    }, [fetchConfig]);

    const handleSave = async () => {
        setError('');
        setMessage('');
        try {
            // Save library scan schedule and enabled status
            await subsonicApiRequest('setConfiguration.view', { key: 'scan_schedule', value: schedule });
            await subsonicApiRequest('setConfiguration.view', { key: 'scan_enabled', value: isEnabled });

            // Save analysis schedule and enabled flag
            await subsonicApiRequest('setConfiguration.view', { key: 'analysis_schedule', value: analysisSchedule });
            await subsonicApiRequest('setConfiguration.view', { key: 'analysis_enabled', value: analysisEnabled });

            // Save clustering schedule and enabled flag
            await subsonicApiRequest('setConfiguration.view', { key: 'clustering_schedule', value: clusteringSchedule });
            await subsonicApiRequest('setConfiguration.view', { key: 'clustering_enabled', value: clusteringEnabled });
            
            // Notify backend and reload config so scheduler is restarted server-side
            onConfigChange();
            // Re-fetch the saved configuration to show an accurate summary
            await fetchConfig();

            const scanStatus = `${isEnabled ? 'enabled' : 'disabled'} (${schedule})`;
            const analysisStatus = `${analysisEnabled ? 'enabled' : 'disabled'} (${analysisSchedule})`;
            const clusteringStatus = `${clusteringEnabled ? 'enabled' : 'disabled'} (${clusteringSchedule})`;

            setMessage(`Scheduler reloaded. Scan: ${scanStatus}. Analysis: ${analysisStatus}. Clustering: ${clusteringStatus}`);
            // Keep the message visible for a bit longer so admins can read the summary
            setTimeout(() => setMessage(''), 10000);
        } catch (err) {
            setError(err.message || 'Failed to save settings.');
        }
    };

    return (
        <div className="bg-gray-800 p-4 sm:p-6 rounded-lg">
            <h3 className="text-xl font-bold mb-4">Music Library Automatic scanning</h3>
            <div className="space-y-4">
                <div className="flex items-center">
                    <input
                        type="checkbox"
                        id="scan-enabled"
                        checked={isEnabled}
                        onChange={(e) => setIsEnabled(e.target.checked)}
                        className="w-5 h-5 text-teal-600 bg-gray-700 border-gray-600 rounded focus:ring-teal-500"
                    />
                    <label htmlFor="scan-enabled" className="ml-3 text-sm font-medium text-gray-300">Enable Music Library Automatic scanning</label>
                </div>
                <div>
                    <label htmlFor="scan-schedule" className="block text-sm font-medium text-gray-300">Cron Schedule</label>
                     <input
                        type="text"
                        id="scan-schedule"
                        value={schedule}
                        onChange={(e) => setSchedule(e.target.value)}
                        placeholder="0 2 * * *"
                        className="w-full p-2 mt-1 bg-gray-700 rounded border border-gray-600 font-mono"
                        disabled={!isEnabled}
                    />
                    <p className="text-xs text-gray-400 mt-1">Standard cron format. Default is '0 2 * * *' (2 AM daily).</p>
                </div>

                <hr className="border-gray-700" />

                <div>
                    <h4 className="text-lg font-semibold text-gray-200">Scheduled Analysis</h4>
                    <div className="flex items-center mt-2">
                        <input
                            type="checkbox"
                            id="analysis-enabled"
                            checked={analysisEnabled}
                            onChange={(e) => setAnalysisEnabled(e.target.checked)}
                            className="w-5 h-5 text-teal-600 bg-gray-700 border-gray-600 rounded focus:ring-teal-500"
                        />
                        <label htmlFor="analysis-enabled" className="ml-3 text-sm font-medium text-gray-300">Enable scheduled analysis</label>
                    </div>
                    <div className="mt-2">
                        <label htmlFor="analysis-schedule" className="block text-sm font-medium text-gray-300">Cron Schedule</label>
                        <input
                            type="text"
                            id="analysis-schedule"
                            value={analysisSchedule}
                            onChange={(e) => setAnalysisSchedule(e.target.value)}
                            placeholder="0 2 * * 0-5"
                            className="w-full p-2 mt-1 bg-gray-700 rounded border border-gray-600 font-mono"
                            disabled={!analysisEnabled}
                        />
                        <p className="text-xs text-gray-400 mt-1">Default: '0 2 * * 0-5' (2 AM nightly, excluding Saturday).</p>
                    </div>
                </div>

                <hr className="border-gray-700" />

                <div>
                    <h4 className="text-lg font-semibold text-gray-200">Scheduled Clustering</h4>
                    <div className="flex items-center mt-2">
                        <input
                            type="checkbox"
                            id="clustering-enabled"
                            checked={clusteringEnabled}
                            onChange={(e) => setClusteringEnabled(e.target.checked)}
                            className="w-5 h-5 text-teal-600 bg-gray-700 border-gray-600 rounded focus:ring-teal-500"
                        />
                        <label htmlFor="clustering-enabled" className="ml-3 text-sm font-medium text-gray-300">Enable scheduled clustering</label>
                    </div>
                    <div className="mt-2">
                        <label htmlFor="clustering-schedule" className="block text-sm font-medium text-gray-300">Cron Schedule</label>
                        <input
                            type="text"
                            id="clustering-schedule"
                            value={clusteringSchedule}
                            onChange={(e) => setClusteringSchedule(e.target.value)}
                            placeholder="0 2 * * 6"
                            className="w-full p-2 mt-1 bg-gray-700 rounded border border-gray-600 font-mono"
                            disabled={!clusteringEnabled}
                        />
                        <p className="text-xs text-gray-400 mt-1">Default: '0 2 * * 6' (2 AM on Saturdays).</p>
                    </div>
                </div>
            </div>
             <div className="mt-4 flex justify-end">
                <button onClick={handleSave} className="bg-teal-500 hover:bg-teal-600 text-white font-bold py-2 px-4 rounded">Save Settings</button>
            </div>
            {message && <p className="text-green-400 mt-2">{message}</p>}
            {error && <p className="text-red-500 mt-2">{error}</p>}
        </div>
    );
}

export default AutoScanManagement;