// Suggested path: music-server-frontend/src/components/UserSettings.jsx
import React, { useState, useEffect } from 'react';

const API_BASE = process.env.REACT_APP_API_BASE || '';

export function UserSettings({ credentials }) {
    const [settings, setSettings] = useState({
        enabled: false,
        format: 'mp3',
        bitrate: 128
    });
    const [loading, setLoading] = useState(true);
    const [saving, setSaving] = useState(false);
    const [error, setError] = useState('');
    const [success, setSuccess] = useState('');

    useEffect(() => {
        loadSettings();
    }, []);

    const loadSettings = async () => {
        try {
            const token = localStorage.getItem('token');
            const response = await fetch(`${API_BASE}/api/v1/user/settings/transcoding`, {
                headers: {
                    'Authorization': `Bearer ${token}`
                }
            });

            if (!response.ok) {
                throw new Error('Failed to load settings');
            }

            const data = await response.json();
            setSettings(data);
            setLoading(false);
        } catch (err) {
            setError('Failed to load settings: ' + err.message);
            setLoading(false);
        }
    };

    const saveSettings = async () => {
        setSaving(true);
        setError('');
        setSuccess('');

        try {
            const token = localStorage.getItem('token');
            const response = await fetch(`${API_BASE}/api/v1/user/settings/transcoding`, {
                method: 'POST',
                headers: {
                    'Authorization': `Bearer ${token}`,
                    'Content-Type': 'application/json'
                },
                body: JSON.stringify(settings)
            });

            if (!response.ok) {
                const errorData = await response.json();
                throw new Error(errorData.error || 'Failed to save settings');
            }

            setSuccess('Settings saved successfully!');
            setTimeout(() => setSuccess(''), 3000);
        } catch (err) {
            setError('Failed to save settings: ' + err.message);
        } finally {
            setSaving(false);
        }
    };

    if (loading) {
        return (
            <div className="flex justify-center items-center py-20">
                <svg className="animate-spin h-8 w-8 text-accent-400" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
                    <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"></circle>
                    <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                </svg>
            </div>
        );
    }

    return (
        <div className="max-w-4xl mx-auto animate-fade-in">
            <div className="glass rounded-2xl shadow-2xl p-4 sm:p-6 md:p-8">
                    {/* Header */}
                    <div className="mb-6">
                        <h2 className="text-2xl sm:text-3xl font-bold text-white mb-2">User Settings</h2>
                        <p className="text-sm text-gray-400">Configure your personal audio streaming preferences</p>
                    </div>

                    {/* Messages */}
                    {error && (
                        <div className="bg-red-500/10 border border-red-500/50 rounded-lg p-3 mb-4 animate-fade-in">
                            <p className="text-red-400 text-sm flex items-center gap-2">
                                <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"></path>
                                </svg>
                                {error}
                            </p>
                        </div>
                    )}
                    
                    {success && (
                        <div className="bg-green-500/10 border border-green-500/50 rounded-lg p-3 mb-4 animate-fade-in">
                            <p className="text-green-400 text-sm flex items-center gap-2">
                                <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"></path>
                                </svg>
                                {success}
                            </p>
                        </div>
                    )}

                    {/* Audio Transcoding Section */}
                    <div className="space-y-6">
                        <div className="border-b border-dark-600 pb-4">
                            <h3 className="text-xl font-semibold text-white mb-2">Audio Transcoding</h3>
                            <p className="text-sm text-gray-400">
                                Convert audio files on-the-fly to reduce bandwidth usage and improve compatibility.
                                Note: Transcoding requires FFmpeg to be installed on the server.
                            </p>
                        </div>

                        {/* Enable Transcoding */}
                        <div className="flex items-center justify-between p-4 bg-dark-750 rounded-lg">
                            <div>
                                <label className="text-white font-medium">Enable Transcoding</label>
                                <p className="text-sm text-gray-400 mt-1">
                                    Automatically transcode audio during streaming
                                </p>
                            </div>
                            <button
                                onClick={() => setSettings({...settings, enabled: !settings.enabled})}
                                className={`relative inline-flex h-8 w-14 items-center rounded-full transition-colors ${
                                    settings.enabled ? 'bg-accent-500' : 'bg-dark-600'
                                }`}
                            >
                                <span
                                    className={`inline-block h-6 w-6 transform rounded-full bg-white transition-transform ${
                                        settings.enabled ? 'translate-x-7' : 'translate-x-1'
                                    }`}
                                />
                            </button>
                        </div>

                        {/* Format Selection */}
                        <div className="space-y-2">
                            <label className="block text-white font-medium">Audio Format</label>
                            <select
                                value={settings.format}
                                onChange={(e) => setSettings({...settings, format: e.target.value})}
                                disabled={!settings.enabled}
                                className="w-full p-3 bg-dark-700 rounded-lg border border-dark-600 focus:outline-none focus:border-accent-500 focus:ring-2 focus:ring-accent-500/20 text-white transition-all disabled:opacity-50 disabled:cursor-not-allowed"
                            >
                                <option value="mp3">MP3 (Universal compatibility)</option>
                                <option value="ogg">OGG Vorbis (Good quality/size ratio)</option>
                                <option value="aac">AAC (Apple devices)</option>
                                <option value="opus">Opus (Best quality at low bitrates)</option>
                            </select>
                            <p className="text-xs text-gray-500">Choose the output format for transcoded audio</p>
                        </div>

                        {/* Bitrate Selection */}
                        <div className="space-y-2">
                            <label className="block text-white font-medium">Bitrate (kbps)</label>
                            <div className="space-y-3">
                                <input
                                    type="range"
                                    min="64"
                                    max="320"
                                    step="32"
                                    value={settings.bitrate}
                                    onChange={(e) => setSettings({...settings, bitrate: parseInt(e.target.value)})}
                                    disabled={!settings.enabled}
                                    className="w-full h-2 bg-dark-700 rounded-lg appearance-none cursor-pointer accent-accent-500 disabled:opacity-50 disabled:cursor-not-allowed"
                                />
                                <div className="flex justify-between text-sm text-gray-400">
                                    <span>64</span>
                                    <span className="text-accent-400 font-semibold">{settings.bitrate} kbps</span>
                                    <span>320</span>
                                </div>
                            </div>
                            <p className="text-xs text-gray-500">
                                Lower bitrates reduce bandwidth but may affect audio quality
                            </p>
                        </div>

                        {/* Quality Guide */}
                        <div className="bg-dark-750 rounded-lg p-4 space-y-2">
                            <h4 className="text-white font-medium text-sm">Quality Guide</h4>
                            <div className="grid grid-cols-1 sm:grid-cols-3 gap-2 text-xs">
                                <div className="text-gray-400">
                                    <span className="text-red-400 font-semibold">64-96 kbps:</span> Low quality, mobile data
                                </div>
                                <div className="text-gray-400">
                                    <span className="text-yellow-400 font-semibold">128-192 kbps:</span> Good quality, balanced
                                </div>
                                <div className="text-gray-400">
                                    <span className="text-green-400 font-semibold">256-320 kbps:</span> High quality, more bandwidth
                                </div>
                            </div>
                        </div>
                    </div>

                {/* Actions */}
                <div className="flex flex-col sm:flex-row justify-end gap-3 mt-8 pt-6 border-t border-dark-600">
                    <button 
                        onClick={saveSettings}
                        disabled={saving}
                        className="w-full sm:w-auto px-6 py-2.5 rounded-lg bg-gradient-accent text-white font-semibold transition-all shadow-lg hover:shadow-glow disabled:opacity-50 disabled:cursor-not-allowed flex items-center justify-center gap-2"
                    >
                        {saving && (
                            <svg className="animate-spin h-5 w-5" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
                                <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"></circle>
                                <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                            </svg>
                        )}
                        Save Settings
                    </button>
                </div>
            </div>
        </div>
    );
}

export default UserSettings;
