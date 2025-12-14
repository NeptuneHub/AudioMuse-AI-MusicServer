// Suggested path: music-server-frontend/src/components/TextSongSearch.jsx
import React, { useState, useEffect } from 'react';
import { clapSearch, getClapTopQueries } from '../api';

function TextSongSearch({ onNavigate }) {
    const [query, setQuery] = useState('');
    const [isSearching, setIsSearching] = useState(false);
    const [topQueries, setTopQueries] = useState([]);
    const [isLoadingQueries, setIsLoadingQueries] = useState(true);
    const [error, setError] = useState(null);

    useEffect(() => {
        const fetchTopQueries = async () => {
            try {
                setIsLoadingQueries(true);
                const response = await getClapTopQueries();
                if (response.ready && response.queries) {
                    setTopQueries(response.queries);
                }
            } catch (err) {
                console.error('Failed to fetch top queries:', err);
                setError('Failed to load example queries');
            } finally {
                setIsLoadingQueries(false);
            }
        };

        fetchTopQueries();
    }, []);

    const handleSearch = async (searchQuery = query) => {
        if (!searchQuery.trim()) return;

        try {
            setIsSearching(true);
            setError(null);
            
            const response = await clapSearch(searchQuery.trim(), 50);
            
            if (response.songs && response.songs.length > 0) {
                // Navigate to songs view with the search results
                onNavigate({ 
                    page: 'songs', 
                    title: `Text Search: "${response.query}"`,
                    filter: { 
                        type: 'clap-search',
                        results: response.songs,
                        query: response.query 
                    }
                });
            } else {
                setError('No songs found for this query');
            }
        } catch (err) {
            console.error('Search failed:', err);
            setError('Search failed. Please try again.');
        } finally {
            setIsSearching(false);
        }
    };

    const handleKeyPress = (e) => {
        if (e.key === 'Enter') {
            handleSearch();
        }
    };

    const handleExampleClick = (exampleQuery) => {
        setQuery(exampleQuery);
        handleSearch(exampleQuery);
    };

    return (
        <div className="max-w-5xl mx-auto">
            <div className="mb-8">
                <h1 className="text-3xl font-bold mb-2 bg-gradient-to-r from-accent-400 to-accent-600 bg-clip-text text-transparent">
                    Text Song Search
                </h1>
                <p className="text-gray-400">
                    Describe the music you're looking for using natural language
                </p>
            </div>

            {/* Search Input */}
            <div className="bg-dark-700 rounded-xl p-6 mb-8 shadow-xl border border-dark-600">
                <div className="flex flex-col sm:flex-row gap-3">
                    <input
                        type="text"
                        value={query}
                        onChange={(e) => setQuery(e.target.value)}
                        onKeyPress={handleKeyPress}
                        placeholder="e.g., dreamy chant indie pop, groovy falsetto funk, sad blues raspy..."
                        className="flex-1 px-4 py-3 bg-dark-600 text-white rounded-lg focus:outline-none focus:ring-2 focus:ring-accent-500 border border-dark-500 placeholder-gray-500"
                        disabled={isSearching}
                    />
                    <button
                        onClick={() => handleSearch()}
                        disabled={isSearching || !query.trim()}
                        className="px-6 py-3 bg-accent-500 hover:bg-accent-600 disabled:bg-dark-600 disabled:text-gray-500 text-white font-semibold rounded-lg transition-all duration-300 shadow-md hover:shadow-lg disabled:cursor-not-allowed"
                    >
                        {isSearching ? (
                            <span className="flex items-center gap-2">
                                <svg className="animate-spin h-5 w-5" fill="none" viewBox="0 0 24 24">
                                    <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"></circle>
                                    <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                                </svg>
                                Searching...
                            </span>
                        ) : (
                            'Search'
                        )}
                    </button>
                </div>
                
                {error && (
                    <div className="mt-4 p-3 bg-red-500/10 border border-red-500/50 rounded-lg text-red-400">
                        {error}
                    </div>
                )}
            </div>

            {/* Example Queries */}
            <div className="bg-dark-700 rounded-xl p-6 shadow-xl border border-dark-600">
                <h2 className="text-xl font-semibold mb-4 text-gray-200">
                    Example Searches
                </h2>
                
                {isLoadingQueries ? (
                    <div className="flex items-center justify-center py-8">
                        <svg className="animate-spin h-8 w-8 text-accent-500" fill="none" viewBox="0 0 24 24">
                            <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"></circle>
                            <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                        </svg>
                    </div>
                ) : topQueries.length > 0 ? (
                    <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-2">
                        {topQueries.slice(0, 30).map((exampleQuery, idx) => (
                            <button
                                key={idx}
                                onClick={() => handleExampleClick(exampleQuery)}
                                className="px-3 py-2 bg-dark-600 hover:bg-accent-500/20 hover:border-accent-500/50 text-gray-300 hover:text-accent-400 rounded-lg transition-all duration-200 text-left text-sm border border-dark-500"
                            >
                                {exampleQuery}
                            </button>
                        ))}
                    </div>
                ) : (
                    <p className="text-gray-400 text-center py-4">
                        No example queries available
                    </p>
                )}
            </div>

            {/* Info Section */}
            <div className="mt-8 bg-dark-700/50 rounded-xl p-6 border border-dark-600">
                <h3 className="text-lg font-semibold mb-3 text-gray-200">
                    💡 How to use
                </h3>
                <ul className="space-y-2 text-gray-400 text-sm">
                    <li className="flex items-start gap-2">
                        <span className="text-accent-500 mt-0.5">•</span>
                        <span>Combine genres, moods, vocal styles, and instruments to find specific songs</span>
                    </li>
                    <li className="flex items-start gap-2">
                        <span className="text-accent-500 mt-0.5">•</span>
                        <span>Examples: "happy energetic pop", "dark atmospheric metal", "slow jazz piano"</span>
                    </li>
                    <li className="flex items-start gap-2">
                        <span className="text-accent-500 mt-0.5">•</span>
                        <span>Click any example above to try it out</span>
                    </li>
                </ul>
            </div>
        </div>
    );
}

export default TextSongSearch;
