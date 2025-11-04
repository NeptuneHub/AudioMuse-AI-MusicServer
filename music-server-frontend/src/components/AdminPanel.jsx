import React from 'react';
import LibraryManagement from './admin/LibraryManagement';
import AutoScanManagement from './admin/AutoScanManagement';
import AIConfigManagement from './admin/AIConfigManagement';
import UserManagement from './admin/UserManagement';
import SonicAnalysisPanel from './admin/SonicAnalysisPanel';
import ApiKeyManagement from './admin/ApiKeyManagement';

export default function AdminPanel({ onConfigChange }) {
	return (
		<div className="space-y-8">
            <LibraryManagement onConfigChange={onConfigChange} />
            <SonicAnalysisPanel />
            <AutoScanManagement onConfigChange={onConfigChange} />
            <AIConfigManagement onConfigChange={onConfigChange} />
            <ApiKeyManagement />
            <UserManagement />
        </div>
	);
}

