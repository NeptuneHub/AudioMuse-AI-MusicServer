import React, { useState, useEffect, useCallback } from 'react';
import Modal from '../Modal';
import { subsonicFetch } from '../../api';

const UserFormModal = ({ onClose, onSubmit, title }) => {
    const [username, setUsername] = useState('');
    const [password, setPassword] = useState('');
    const [isAdmin, setIsAdmin] = useState(false);

    const handleSubmit = (e) => {
        e.preventDefault();
        onSubmit({ username, password, is_admin: isAdmin });
    };

    return (
        <Modal onClose={onClose}>
            <h3 className="text-xl font-bold mb-4">{title}</h3>
            <form onSubmit={handleSubmit}>
                <div className="mb-4">
                    <label className="block text-gray-400 mb-2">Username</label>
                    <input type="text" value={username} onChange={e => setUsername(e.target.value)} className="w-full p-2 bg-gray-700 rounded" required/>
                </div>
                <div className="mb-4">
                    <label className="block text-gray-400 mb-2">Password</label>
                    <input type="password" value={password} onChange={e => setPassword(e.target.value)} className="w-full p-2 bg-gray-700 rounded" required />
                </div>
                <div className="mb-4 flex items-center">
                    <input type="checkbox" checked={isAdmin} onChange={e => setIsAdmin(e.target.checked)} id="isAdminCheck" className="w-4 h-4 text-teal-600 bg-gray-700 border-gray-600 rounded focus:ring-teal-500" />
                    <label htmlFor="isAdminCheck" className="ml-2 text-sm font-medium text-gray-300">Is Admin?</label>
                </div>
                <div className="flex flex-col sm:flex-row justify-end space-y-2 sm:space-y-0 sm:space-x-4">
                    <button type="button" onClick={onClose} className="border-2 border-gray-500 text-gray-400 bg-gray-500/10 hover:bg-gray-500/20 hover:scale-105 transition-all font-bold py-2 px-4 rounded-lg">Cancel</button>
                    <button type="submit" className="border-2 border-teal-500 text-teal-400 bg-teal-500/10 hover:bg-teal-500/20 hover:scale-105 transition-all font-bold py-2 px-4 rounded-lg">Save</button>
                </div>
            </form>
        </Modal>
    );
};

const PasswordEditModal = ({ user, onClose, onSubmit }) => {
	const [password, setPassword] = useState('');
	const handleSubmit = (e) => {
		e.preventDefault();
		onSubmit(user.username, password);
	};
	return (
		<Modal onClose={onClose}>
			<h3 className="text-xl font-bold mb-4">Edit Password for {user.username}</h3>
			<form onSubmit={handleSubmit}>
				<div className="mb-4">
					<label className="block text-gray-400 mb-2">New Password</label>
					<input
						type="password"
						value={password}
						onChange={(e) => setPassword(e.target.value)}
						className="w-full p-2 bg-gray-700 rounded"
						required
					/>
				</div>
                <div className="flex flex-col sm:flex-row justify-end space-y-2 sm:space-y-0 sm:space-x-4">
                    <button type="button" onClick={onClose} className="border-2 border-gray-500 text-gray-400 bg-gray-500/10 hover:bg-gray-500/20 hover:scale-105 transition-all font-bold py-2 px-4 rounded-lg">Cancel</button>
                    <button type="submit" className="border-2 border-teal-500 text-teal-400 bg-teal-500/10 hover:bg-teal-500/20 hover:scale-105 transition-all font-bold py-2 px-4 rounded-lg">Update Password</button>
                </div>
			</form>
		</Modal>
	);
};

function UserManagement() {
	const [users, setUsers] = useState([]);
	const [editingUser, setEditingUser] = useState(null);
    const [isCreatingUser, setIsCreatingUser] = useState(false);
	const [error, setError] = useState('');
    const [successMessage, setSuccessMessage] = useState('');

    const subsonicApiRequest = useCallback(async (endpoint, params = {}) => {
        return await subsonicFetch(endpoint, params);
    }, []);

	const fetchUsers = useCallback(async () => {
		try {
			const data = await subsonicApiRequest('getUsers.view');
			const userList = data?.users?.user || [];
			setUsers(userList.map(u => ({ username: u.username, is_admin: u.adminRole })));
		} catch (err) {
			setError(err.message || 'Failed to fetch users');
		}
	}, [subsonicApiRequest]);

	useEffect(() => {
		fetchUsers();
	}, [fetchUsers]);

    const handleCreate = async (userData) => {
        setError('');
        setSuccessMessage('');
        try {
            await subsonicApiRequest('createUser.view', {
                username: userData.username,
                password: userData.password,
                adminRole: userData.is_admin,
            });
            setIsCreatingUser(false);
            setSuccessMessage(`User ${userData.username} created successfully.`);
            fetchUsers();
        } catch (err) {
            setError(err.message || 'Failed to create user.');
        }
    };

	const handlePasswordChange = async (username, password) => {
        setError('');
        setSuccessMessage('');
        try {
            await subsonicApiRequest('updateUser.view', { username, password });
            setEditingUser(null);
            setSuccessMessage('Password updated successfully.');
        } catch (err) {
            setError(err.message || 'Failed to update password.');
        }
    };

	const handleDelete = async (username) => {
        setError('');
        setSuccessMessage('');
		if (window.confirm(`Are you sure you want to delete user: ${username}?`)) {
			try {
                await subsonicApiRequest('deleteUser.view', { username });
				setSuccessMessage('User deleted successfully.');
				fetchUsers();
			} catch (err) {
                setError(err.message || 'Failed to delete user.');
			}
		}
	};

	return (
		<div className="bg-gray-800 p-4 sm:p-6 rounded-lg">
			<div className="flex flex-col sm:flex-row justify-between items-start sm:items-center mb-4 space-y-2 sm:space-y-0">
				<h3 className="text-xl font-bold">User Management</h3>
				<button onClick={() => setIsCreatingUser(true)} className="border-2 border-purple-500 text-purple-400 bg-purple-500/10 hover:bg-purple-500/20 hover:scale-105 transition-all font-bold py-2 px-4 rounded-lg">Create User</button>
			</div>
			{error && <p className="text-red-500 mb-4 p-3 bg-red-900/50 rounded">{error}</p>}
            {successMessage && <p className="text-green-400 mb-4 p-3 bg-green-900/50 rounded">{successMessage}</p>}
			<div className="overflow-x-auto">
				<table className="min-w-full text-sm text-left text-gray-400">
					<thead className="text-xs text-gray-300 uppercase bg-gray-700">
						<tr>
							<th scope="col" className="px-6 py-3">Username</th>
							<th scope="col" className="px-6 py-3">Admin</th>
							<th scope="col" className="px-6 py-3 text-right">Actions</th>
						</tr>
					</thead>
					<tbody>
						{users.map(user => (
							<tr key={user.username} className="bg-gray-800 border-b border-gray-700 hover:bg-gray-600">
								<td className="px-6 py-4 font-medium text-white">{user.username}</td>
								<td className="px-6 py-4">{user.is_admin ? 'Yes' : 'No'}</td>
								<td className="px-6 py-4 text-right space-x-2 whitespace-nowrap">
									<button onClick={() => setEditingUser(user)} className="border-2 border-blue-500 text-blue-400 bg-blue-500/10 hover:bg-blue-500/20 hover:scale-105 transition-all px-2 py-1 rounded-lg text-sm">Edit Password</button>
									<button onClick={() => handleDelete(user.username)} className="border-2 border-red-500 text-red-400 bg-red-500/10 hover:bg-red-500/20 hover:scale-105 transition-all px-2 py-1 rounded-lg text-sm">Delete</button>
								</td>
							</tr>
						))}
					</tbody>
				</table>
			</div>
            {isCreatingUser && (
                <UserFormModal
                    onClose={() => setIsCreatingUser(false)}
                    onSubmit={handleCreate}
                    title="Create New User"
                />
            )}
			{editingUser && (
				<PasswordEditModal
					user={editingUser}
					onClose={() => setEditingUser(null)}
					onSubmit={handlePasswordChange}
				/>
			)}
		</div>
	);
}

export default UserManagement;