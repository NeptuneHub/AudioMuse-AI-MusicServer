import React, { useState, useEffect, useCallback } from 'react';

function AutoScanManagement({ onConfigChange }) {
    const [schedule, setSchedule] = useState('');
    const [isEnabled, setIsEnabled] = useState(false);
    const [message, setMessage] = useState('');
    const [error, setError] = useState('');

    const subsonicApiRequest = useCallback(async (endpoint, params = {}) => {
        const token = localStorage.getItem('token');
        const query = new URLSearchParams(params);
        query.append('f', 'json');

        const response = await fetch(`/rest/${endpoint}?${query.toString()}`, {
            headers: { 'Authorization': `Bearer ${token}` }
        });
        const data = await response.json();
        const subsonicResponse = data["subsonic-response"];

        if (!response.ok || subsonicResponse.status === 'failed') {
            const error = subsonicResponse?.error;
            throw new Error(error?.message || `Server error: ${response.status}`);
        }
        return subsonicResponse;
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
            // Save schedule and enabled status as two separate API calls
            await subsonicApiRequest('setConfiguration.view', { key: 'scan_schedule', value: schedule });
            await subsonicApiRequest('setConfiguration.view', { key: 'scan_enabled', value: isEnabled });
            
            setMessage('Auto-scan settings saved successfully! Restart the server for changes to take effect.');
            onConfigChange();
            setTimeout(() => setMessage(''), 5000);
        } catch (err) {
            setError(err.message || 'Failed to save settings.');
        }
    };

    return (
        <div className="bg-gray-800 p-4 sm:p-6 rounded-lg">
            <h3 className="text-xl font-bold mb-4">Automatic Library Scanning</h3>
            <div className="space-y-4">
                <div className="flex items-center">
                    <input
                        type="checkbox"
                        id="scan-enabled"
                        checked={isEnabled}
                        onChange={(e) => setIsEnabled(e.target.checked)}
                        className="w-5 h-5 text-teal-600 bg-gray-700 border-gray-600 rounded focus:ring-teal-500"
                    />
                    <label htmlFor="scan-enabled" className="ml-3 text-sm font-medium text-gray-300">Enable automatic scanning</label>
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