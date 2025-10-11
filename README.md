# AudioMuse-AI-MusicServer

<p align="center">
  <img src="https://github.com/NeptuneHub/audiomuse-ai-plugin/blob/master/audiomuseai.png?raw=true" alt="AudioMuse-AI Logo" width="480">
</p>


Prototype music server built on the Open Subsonic API, designed to showcase AudioMuse-AI's advanced sonic analysis capabilities.

> 👉 Experience the **Sonic Analysis potentiality** with the [AudioMuse-AI-MusicServer Demo](https://github.com/NeptuneHub/AudioMuse-AI/issues/94)!  
> _Available for a limited time ⌛_

**The full list or AudioMuse-AI related repository are:** 
  > * [AudioMuse-AI](https://github.com/NeptuneHub/AudioMuse-AI): the core application, it run Flask and Worker containers to actually run all the feature;
  > * [AudioMuse-AI Helm Chart](https://github.com/NeptuneHub/AudioMuse-AI-helm): helm chart for easy installation on Kubernetes;
  > * [AudioMuse-AI Plugin for Jellyfin](https://github.com/NeptuneHub/audiomuse-ai-plugin): Jellyfin Plugin;
  > * [AudioMuse-AI MusicServer](https://github.com/NeptuneHub/AudioMuse-AI-MusicServer): **Experimental** Open Subosnic like Music Sever with integrated sonic functionality.

## Container Deployment

**Pre-built Docker containers are available!** 

You can run AudioMuse-AI-MusicServer using our automatically built and published Docker containers from GitHub Container Registry. This is the easiest way to get started without needing to compile anything.

```bash
docker pull ghcr.io/neptunehub/audiomuse-ai-musicserver:latest
```

For detailed container usage instructions, deployment options, and release information, see: **[CONTAINER_RELEASE.md](CONTAINER_RELEASE.md)**

The containers include both the backend API server and frontend web interface, with automatic nightly builds and easy version management.

After deploying AudioMsue-AI-Music server it could be reached BOTH from this url:

* http://localhost:8080/

## Music server configuration

The first login can be don with:
* User: admin
* passwoord: admin

The configuration needed is go in the admin tab and:
* add the path of the song, and start the scanning, depending from the size of the library could takes several minutes. This is just to add the  song to the mediaserver
* configure the path of `AudioMuse-AI` (the core contianer)

After both of this point done, you can start the Sonic Analysis diectly from the Music Server, after the analysis is completed you can run the integrated Sonic Analysis function that now are:
* Instnat Mix
* Sonic Path

**IMPORTANT:** This is a Open Subsonic API compliant server (or it should be), so you need to configure AudioMuse-AI with the **navidrome** deployment, setting url, user and password correctly.

# Developer
This is instruction to run both backend and frontned on your developing envirorment

## Compile and run backend
Going in /music-server-backend/

```
go mod init music-server-backend
go mod tidy
go build -o music-server
./music-server
```

API will be reacheable on http://localhost:8080/rest/

API actually exposed:
```
[GIN-debug] GET    /rest/ping.view           --> main.subsonicPing (3 handlers)
[GIN-debug] GET    /rest/getOpenSubsonicExtensions.view --> main.subsonicGetOpenSubsonicExtensions (3 handlers)
[GIN-debug] GET    /rest/getLicense.view     --> main.subsonicGetLicense (4 handlers)
[GIN-debug] GET    /rest/stream.view         --> main.subsonicStream (4 handlers)
[GIN-debug] GET    /rest/scrobble.view       --> main.subsonicScrobble (4 handlers)
[GIN-debug] GET    /rest/getArtists.view     --> main.subsonicGetArtists (4 handlers)
[GIN-debug] GET    /rest/getAlbumList2.view  --> main.subsonicGetAlbumList2 (4 handlers)
[GIN-debug] GET    /rest/getPlaylists.view   --> main.subsonicGetPlaylists (4 handlers)
[GIN-debug] GET    /rest/getPlaylist.view    --> main.subsonicGetPlaylist (4 handlers)
[GIN-debug] GET    /rest/createPlaylist.view --> main.subsonicCreatePlaylist (4 handlers)
[GIN-debug] POST   /rest/createPlaylist.view --> main.subsonicCreatePlaylist (4 handlers)
[GIN-debug] PUT    /rest/createPlaylist.view --> main.subsonicCreatePlaylist (4 handlers)
[GIN-debug] PATCH  /rest/createPlaylist.view --> main.subsonicCreatePlaylist (4 handlers)
[GIN-debug] HEAD   /rest/createPlaylist.view --> main.subsonicCreatePlaylist (4 handlers)
[GIN-debug] OPTIONS /rest/createPlaylist.view --> main.subsonicCreatePlaylist (4 handlers)
[GIN-debug] DELETE /rest/createPlaylist.view --> main.subsonicCreatePlaylist (4 handlers)
[GIN-debug] CONNECT /rest/createPlaylist.view --> main.subsonicCreatePlaylist (4 handlers)
[GIN-debug] TRACE  /rest/createPlaylist.view --> main.subsonicCreatePlaylist (4 handlers)
[GIN-debug] GET    /rest/updatePlaylist.view --> main.subsonicUpdatePlaylist (4 handlers)
[GIN-debug] GET    /rest/deletePlaylist.view --> main.subsonicDeletePlaylist (4 handlers)
[GIN-debug] GET    /rest/getAlbum.view       --> main.subsonicGetAlbum (4 handlers)
[GIN-debug] GET    /rest/search2.view        --> main.subsonicSearch2 (4 handlers)
[GIN-debug] GET    /rest/search3.view        --> main.subsonicSearch2 (4 handlers)
[GIN-debug] GET    /rest/getSong.view        --> main.subsonicGetSong (4 handlers)
[GIN-debug] GET    /rest/getRandomSongs.view --> main.subsonicGetRandomSongs (4 handlers)
[GIN-debug] GET    /rest/getCoverArt.view    --> main.subsonicGetCoverArt (4 handlers)
[GIN-debug] GET    /rest/startScan.view      --> main.subsonicStartScan (4 handlers)
[GIN-debug] POST   /rest/startScan.view      --> main.subsonicStartScan (4 handlers)
[GIN-debug] PUT    /rest/startScan.view      --> main.subsonicStartScan (4 handlers)
[GIN-debug] PATCH  /rest/startScan.view      --> main.subsonicStartScan (4 handlers)
[GIN-debug] HEAD   /rest/startScan.view      --> main.subsonicStartScan (4 handlers)
[GIN-debug] OPTIONS /rest/startScan.view      --> main.subsonicStartScan (4 handlers)
[GIN-debug] DELETE /rest/startScan.view      --> main.subsonicStartScan (4 handlers)
[GIN-debug] CONNECT /rest/startScan.view      --> main.subsonicStartScan (4 handlers)
[GIN-debug] TRACE  /rest/startScan.view      --> main.subsonicStartScan (4 handlers)
[GIN-debug] GET    /rest/getScanStatus.view  --> main.subsonicGetScanStatus (4 handlers)
[GIN-debug] GET    /rest/getLibraryPaths.view --> main.subsonicGetLibraryPaths (4 handlers)
[GIN-debug] POST   /rest/addLibraryPath.view --> main.subsonicAddLibraryPath (4 handlers)
[GIN-debug] POST   /rest/updateLibraryPath.view --> main.subsonicUpdateLibraryPath (4 handlers)
[GIN-debug] POST   /rest/deleteLibraryPath.view --> main.subsonicDeleteLibraryPath (4 handlers)
[GIN-debug] GET    /rest/getUsers.view       --> main.subsonicGetUsers (4 handlers)
[GIN-debug] GET    /rest/createUser.view     --> main.subsonicCreateUser (4 handlers)
[GIN-debug] GET    /rest/updateUser.view     --> main.subsonicUpdateUser (4 handlers)
[GIN-debug] GET    /rest/deleteUser.view     --> main.subsonicDeleteUser (4 handlers)
[GIN-debug] GET    /rest/changePassword.view --> main.subsonicChangePassword (4 handlers)
[GIN-debug] GET    /rest/getConfiguration.view --> main.subsonicGetConfiguration (4 handlers)
[GIN-debug] GET    /rest/setConfiguration.view --> main.subsonicSetConfiguration (4 handlers)
[GIN-debug] GET    /rest/getSimilarSongs.view --> main.subsonicGetSimilarSongs (4 handlers)
[GIN-debug] GET    /rest/getSongPath.view    --> main.subsonicGetSongPath (4 handlers)
[GIN-debug] GET    /rest/getSonicFingerprint.view --> main.subsonicGetSonicFingerprint (4 handlers)
[GIN-debug] GET    /rest/getApiKey.view      --> main.subsonicGetApiKey (4 handlers)
[GIN-debug] POST   /rest/revokeApiKey.view   --> main.subsonicRevokeApiKey (4 handlers)
[GIN-debug] GET    /rest/startSonicAnalysis.view --> main.subsonicStartSonicAnalysis (4 handlers)
[GIN-debug] POST   /rest/startSonicAnalysis.view --> main.subsonicStartSonicAnalysis (4 handlers)
[GIN-debug] PUT    /rest/startSonicAnalysis.view --> main.subsonicStartSonicAnalysis (4 handlers)
[GIN-debug] PATCH  /rest/startSonicAnalysis.view --> main.subsonicStartSonicAnalysis (4 handlers)
[GIN-debug] HEAD   /rest/startSonicAnalysis.view --> main.subsonicStartSonicAnalysis (4 handlers)
[GIN-debug] OPTIONS /rest/startSonicAnalysis.view --> main.subsonicStartSonicAnalysis (4 handlers)
[GIN-debug] DELETE /rest/startSonicAnalysis.view --> main.subsonicStartSonicAnalysis (4 handlers)
[GIN-debug] CONNECT /rest/startSonicAnalysis.view --> main.subsonicStartSonicAnalysis (4 handlers)
[GIN-debug] TRACE  /rest/startSonicAnalysis.view --> main.subsonicStartSonicAnalysis (4 handlers)
[GIN-debug] GET    /rest/getSonicAnalysisStatus.view --> main.subsonicGetSonicAnalysisStatus (4 handlers)
[GIN-debug] GET    /rest/cancelSonicAnalysis.view --> main.subsonicCancelSonicAnalysis (4 handlers)
[GIN-debug] POST   /rest/cancelSonicAnalysis.view --> main.subsonicCancelSonicAnalysis (4 handlers)
[GIN-debug] PUT    /rest/cancelSonicAnalysis.view --> main.subsonicCancelSonicAnalysis (4 handlers)
[GIN-debug] PATCH  /rest/cancelSonicAnalysis.view --> main.subsonicCancelSonicAnalysis (4 handlers)
[GIN-debug] HEAD   /rest/cancelSonicAnalysis.view --> main.subsonicCancelSonicAnalysis (4 handlers)
[GIN-debug] OPTIONS /rest/cancelSonicAnalysis.view --> main.subsonicCancelSonicAnalysis (4 handlers)
[GIN-debug] DELETE /rest/cancelSonicAnalysis.view --> main.subsonicCancelSonicAnalysis (4 handlers)
[GIN-debug] CONNECT /rest/cancelSonicAnalysis.view --> main.subsonicCancelSonicAnalysis (4 handlers)
[GIN-debug] TRACE  /rest/cancelSonicAnalysis.view --> main.subsonicCancelSonicAnalysis (4 handlers)
[GIN-debug] GET    /rest/startSonicClustering.view --> main.subsonicStartClusteringAnalysis (4 handlers)
[GIN-debug] POST   /rest/startSonicClustering.view --> main.subsonicStartClusteringAnalysis (4 handlers)
[GIN-debug] PUT    /rest/startSonicClustering.view --> main.subsonicStartClusteringAnalysis (4 handlers)
[GIN-debug] PATCH  /rest/startSonicClustering.view --> main.subsonicStartClusteringAnalysis (4 handlers)
[GIN-debug] HEAD   /rest/startSonicClustering.view --> main.subsonicStartClusteringAnalysis (4 handlers)
[GIN-debug] OPTIONS /rest/startSonicClustering.view --> main.subsonicStartClusteringAnalysis (4 handlers)
[GIN-debug] DELETE /rest/startSonicClustering.view --> main.subsonicStartClusteringAnalysis (4 handlers)
[GIN-debug] CONNECT /rest/startSonicClustering.view --> main.subsonicStartClusteringAnalysis (4 handlers)
[GIN-debug] TRACE  /rest/startSonicClustering.view --> main.subsonicStartClusteringAnalysis (4 handlers)
[GIN-debug] POST   /api/v1/user/login        --> main.loginUser (3 handlers)
[GIN-debug] GET    /api/v1/admin/browse      --> main.browseFiles (5 handlers)
[GIN-debug] POST   /api/v1/admin/scan/cancel --> main.cancelAdminScan (5 handlers)
```

## Compile and run frontend
Goining /music-server-frontend/

```
npm install
npm start
```

Frontend will be reacheable on http://localhost:3000/ you can do the first login with admin/admin

**IMPORTANT** as you can see, running the code OUT of the container, you had the front-end on the different port 3000
