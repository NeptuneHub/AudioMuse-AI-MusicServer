import React from 'react';
import LibraryManagement from './admin/LibraryManagement';
import AutoScanManagement from './admin/AutoScanManagement';
import AIConfigManagement from './admin/AIConfigManagement';
import UserManagement from './admin/UserManagement';
import SonicAnalysisPanel from './admin/SonicAnalysisPanel';

export default function AdminPanel({ onConfigChange }) {
	return (
		<div className="grid grid-cols-1 xl:grid-cols-2 gap-8">
            <div className="space-y-8">
                 <LibraryManagement onConfigChange={onConfigChange} />
                 <SonicAnalysisPanel />
                 <AutoScanManagement onConfigChange={onConfigChange} />
                 <AIConfigManagement onConfigChange={onConfigChange} />
            </div>
            <UserManagement />
        </div>
	);
}
