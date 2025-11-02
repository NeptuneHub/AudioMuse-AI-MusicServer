import React, { useState, useEffect, useCallback } from 'react';
import { subsonicFetch } from '../../api';

function AIConfigManagement({ onConfigChange }) {
    const [audiomuseUrl, setAudiomuseUrl] = useState('');
    const [message, setMessage] = useState('');
    const [error, setError] = useState('');

    const subsonicApiRequest = useCallback(async (endpoint, params = {}) => {
        // Use shared helper; credentials not required here because server-side JWT auth will be used
        return await subsonicFetch(endpoint, params);
    }, []);

    const fetchConfig = useCallback(async () => {
        try {
            const data = await subsonicApiRequest('getConfiguration.view');
            const configList = data?.configurations?.configuration || [];
            const allConfigs = Array.isArray(configList) ? configList : [configList].filter(Boolean);
            const urlConfig = allConfigs.find(c => c.name === 'audiomuse_ai_core_url');
            setAudiomuseUrl(urlConfig?.value || '');
        } catch (err) {
            setError(err.message || 'Failed to fetch configuration');
        }
    }, [subsonicApiRequest]);

    useEffect(() => {
        fetchConfig();
    }, [fetchConfig]);

    const handleSave = async () => {
        setError('');
        setMessage('');
        try {
            await subsonicApiRequest('setConfiguration.view', { key: 'audiomuse_ai_core_url', value: audiomuseUrl });
            setMessage('URL saved successfully!');
            onConfigChange();
            setTimeout(() => setMessage(''), 3000);
        } catch (err) {
            setError(err.message || 'Failed to save URL.');
        }
    };

    return (
        <div className="bg-gray-800 p-4 sm:p-6 rounded-lg">
            <h3 className="text-xl font-bold mb-4">AudioMuse-AI Core Integration</h3>
            <div className="space-y-2">
                <label htmlFor="audiomuse-url" className="block text-sm font-medium text-gray-300">Core Service URL</label>
                <input
                    type="text"
                    id="audiomuse-url"
                    value={audiomuseUrl}
                    onChange={(e) => setAudiomuseUrl(e.target.value)}
                    placeholder="http://localhost:8000"
                    className="w-full p-2 bg-gray-700 rounded border border-gray-600"
                />
            </div>
            <div className="mt-4 flex justify-end">
                <button onClick={handleSave} className="border-2 border-teal-500 text-teal-400 bg-teal-500/10 hover:bg-teal-500/20 hover:scale-105 transition-all font-bold py-2 px-4 rounded-lg">Save</button>
            </div>
            {message && <p className="text-green-400 mt-2">{message}</p>}
            {error && <p className="text-red-500 mt-2">{error}</p>}
        </div>
    );
}

export default AIConfigManagement;