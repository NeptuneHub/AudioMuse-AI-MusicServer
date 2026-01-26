# AudioMuse-AI-MusicServer

<p align="center">
  <img src="https://github.com/NeptuneHub/audiomuse-ai-plugin/blob/master/audiomuseai.png?raw=true" alt="AudioMuse-AI Logo" width="480">
</p>


Music Server built on the Open Subsonic API, designed to showcase AudioMuse-AI's advanced sonic analysis capabilities.


**The full list or AudioMuse-AI related repository are:** 
  > * [AudioMuse-AI](https://github.com/NeptuneHub/AudioMuse-AI): the core application, it run Flask and Worker containers to actually run all the feature;
  > * [AudioMuse-AI Helm Chart](https://github.com/NeptuneHub/AudioMuse-AI-helm): helm chart for easy installation on Kubernetes;
  > * [AudioMuse-AI Plugin for Jellyfin](https://github.com/NeptuneHub/audiomuse-ai-plugin): Jellyfin Plugin;
  > * [AudioMuse-AI MusicServer](https://github.com/NeptuneHub/AudioMuse-AI-MusicServer): Open Subosnic like Music Sever with integrated sonic functionality.

## Container Deployment

**Pre-built Docker containers are available!** 

You can run AudioMuse-AI-MusicServer using our automatically built and published Docker containers from GitHub Container Registry. This is the easiest way to get started without needing to compile anything.

```bash
docker run -d \
  --name audiomuse \
  -p 3000:3000 \
  -p 8080:8080 \
  -v /path/to/music:/music \
  -v /path/to/config:/config \
  ghcr.io/neptunehub/audiomuse-ai-musicserver:latest
```

For detailed container usage instructions, deployment options, and release information, see: **[CONTAINER_RELEASE.md](CONTAINER_RELEASE.md)**

The containers include both the backend API server and frontend web interface, with automatic nightly builds and easy version management.

After deploying AudioMsue-AI-MusicServer it could be reached BOTH from this url:

* http://localhost:8080/

## Music server configuration

The first login can be done with:
* User: admin
* password: admin

The configuration needed is go in the admin tab and:
* add the path of the song, and start the scanning, depending from the size of the library could takes several minutes. This is just to add the  song to the mediaserver
* configure the path of `AudioMuse-AI` (the core contianer)

After both of this point done, you can start the Sonic Analysis directly from the Music Server, after the analysis is completed you can run the integrated Sonic Analysis function that now are:
* Instan mix for both Artist and Song
* Sonic Path
* Sonic Fingerprint
* Music Map
* Song Alchemy
* Text Search

**IMPORTANT:** This is a Open Subsonic API compliant server, so you need to configure AudioMuse-AI with the **navidrome** deployment, setting url, user and password correctly.

# Developer
This is instruction to run both backend and frontned on your developing environment

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
[GIN-debug] GET    /rest/ping.view           --> main.subsonicPing (4 handlers)
[GIN-debug] GET    /rest/getOpenSubsonicExtensions.view --> main.subsonicGetOpenSubsonicExtensions (4 handlers)
[GIN-debug] GET    /rest/getLicense.view     --> main.subsonicGetLicense (5 handlers)
[GIN-debug] GET    /rest/stream.view         --> main.subsonicStream (5 handlers)
[GIN-debug] GET    /rest/scrobble.view       --> main.subsonicScrobble (5 handlers)
[GIN-debug] GET    /rest/getArtists.view     --> main.subsonicGetArtists (5 handlers)
[GIN-debug] GET    /rest/getAlbumList2.view  --> main.subsonicGetAlbumList2 (5 handlers)
[GIN-debug] GET    /rest/getPlaylists.view   --> main.subsonicGetPlaylists (5 handlers)
[GIN-debug] GET    /rest/getPlaylist.view    --> main.subsonicGetPlaylist (5 handlers)
[GIN-debug] GET    /rest/createPlaylist.view --> main.subsonicCreatePlaylist (5 handlers)
[GIN-debug] POST   /rest/createPlaylist.view --> main.subsonicCreatePlaylist (5 handlers)
[GIN-debug] PUT    /rest/createPlaylist.view --> main.subsonicCreatePlaylist (5 handlers)
[GIN-debug] PATCH  /rest/createPlaylist.view --> main.subsonicCreatePlaylist (5 handlers)
[GIN-debug] HEAD   /rest/createPlaylist.view --> main.subsonicCreatePlaylist (5 handlers)
[GIN-debug] OPTIONS /rest/createPlaylist.view --> main.subsonicCreatePlaylist (5 handlers)
[GIN-debug] DELETE /rest/createPlaylist.view --> main.subsonicCreatePlaylist (5 handlers)
[GIN-debug] CONNECT /rest/createPlaylist.view --> main.subsonicCreatePlaylist (5 handlers)
[GIN-debug] TRACE  /rest/createPlaylist.view --> main.subsonicCreatePlaylist (5 handlers)
[GIN-debug] GET    /rest/updatePlaylist.view --> main.subsonicUpdatePlaylist (5 handlers)
[GIN-debug] GET    /rest/deletePlaylist.view --> main.subsonicDeletePlaylist (5 handlers)
[GIN-debug] GET    /rest/getAlbum.view       --> main.subsonicGetAlbum (5 handlers)
[GIN-debug] GET    /rest/search2.view        --> main.subsonicSearch2 (5 handlers)
[GIN-debug] GET    /rest/search3.view        --> main.subsonicSearch2 (5 handlers)
[GIN-debug] GET    /rest/getSong.view        --> main.subsonicGetSong (5 handlers)
[GIN-debug] GET    /rest/getRandomSongs.view --> main.subsonicGetRandomSongs (5 handlers)
[GIN-debug] GET    /rest/getSongsByGenre.view --> main.subsonicGetSongsByGenre (5 handlers)
[GIN-debug] GET    /rest/getCoverArt.view    --> main.subsonicGetCoverArt (5 handlers)
[GIN-debug] GET    /rest/startScan.view      --> main.subsonicStartScan (5 handlers)
[GIN-debug] POST   /rest/startScan.view      --> main.subsonicStartScan (5 handlers)
[GIN-debug] PUT    /rest/startScan.view      --> main.subsonicStartScan (5 handlers)
[GIN-debug] PATCH  /rest/startScan.view      --> main.subsonicStartScan (5 handlers)
[GIN-debug] HEAD   /rest/startScan.view      --> main.subsonicStartScan (5 handlers)
[GIN-debug] OPTIONS /rest/startScan.view      --> main.subsonicStartScan (5 handlers)
[GIN-debug] DELETE /rest/startScan.view      --> main.subsonicStartScan (5 handlers)
[GIN-debug] CONNECT /rest/startScan.view      --> main.subsonicStartScan (5 handlers)
[GIN-debug] TRACE  /rest/startScan.view      --> main.subsonicStartScan (5 handlers)
[GIN-debug] GET    /rest/getScanStatus.view  --> main.subsonicGetScanStatus (5 handlers)
[GIN-debug] GET    /rest/getLibraryPaths.view --> main.subsonicGetLibraryPaths (5 handlers)
[GIN-debug] POST   /rest/addLibraryPath.view --> main.subsonicAddLibraryPath (5 handlers)
[GIN-debug] POST   /rest/updateLibraryPath.view --> main.subsonicUpdateLibraryPath (5 handlers)
[GIN-debug] POST   /rest/deleteLibraryPath.view --> main.subsonicDeleteLibraryPath (5 handlers)
[GIN-debug] GET    /rest/getUsers.view       --> main.subsonicGetUsers (5 handlers)
[GIN-debug] GET    /rest/createUser.view     --> main.subsonicCreateUser (5 handlers)
[GIN-debug] GET    /rest/updateUser.view     --> main.subsonicUpdateUser (5 handlers)
[GIN-debug] GET    /rest/deleteUser.view     --> main.subsonicDeleteUser (5 handlers)
[GIN-debug] GET    /rest/changePassword.view --> main.subsonicChangePassword (5 handlers)
[GIN-debug] GET    /rest/getConfiguration.view --> main.subsonicGetConfiguration (5 handlers)
[GIN-debug] GET    /rest/setConfiguration.view --> main.subsonicSetConfiguration (5 handlers)
[GIN-debug] GET    /rest/getSimilarSongs.view --> main.subsonicGetSimilarSongs (5 handlers)
[GIN-debug] GET    /rest/getSongPath.view    --> main.subsonicGetSongPath (5 handlers)
[GIN-debug] GET    /rest/getSonicFingerprint.view --> main.subsonicGetSonicFingerprint (5 handlers)
[GIN-debug] GET    /rest/star.view           --> main.subsonicStar (5 handlers)
[GIN-debug] GET    /rest/unstar.view         --> main.subsonicUnstar (5 handlers)
[GIN-debug] GET    /rest/getStarred.view     --> main.subsonicGetStarred (5 handlers)
[GIN-debug] GET    /rest/getGenres.view      --> main.subsonicGetGenres (5 handlers)
[GIN-debug] GET    /rest/getApiKey.view      --> main.subsonicGetApiKey (5 handlers)
[GIN-debug] POST   /rest/revokeApiKey.view   --> main.subsonicRevokeApiKey (5 handlers)
[GIN-debug] GET    /rest/startSonicAnalysis.view --> main.subsonicStartSonicAnalysis (5 handlers)
[GIN-debug] POST   /rest/startSonicAnalysis.view --> main.subsonicStartSonicAnalysis (5 handlers)
[GIN-debug] PUT    /rest/startSonicAnalysis.view --> main.subsonicStartSonicAnalysis (5 handlers)
[GIN-debug] PATCH  /rest/startSonicAnalysis.view --> main.subsonicStartSonicAnalysis (5 handlers)
[GIN-debug] HEAD   /rest/startSonicAnalysis.view --> main.subsonicStartSonicAnalysis (5 handlers)
[GIN-debug] OPTIONS /rest/startSonicAnalysis.view --> main.subsonicStartSonicAnalysis (5 handlers)
[GIN-debug] DELETE /rest/startSonicAnalysis.view --> main.subsonicStartSonicAnalysis (5 handlers)
[GIN-debug] CONNECT /rest/startSonicAnalysis.view --> main.subsonicStartSonicAnalysis (5 handlers)
[GIN-debug] TRACE  /rest/startSonicAnalysis.view --> main.subsonicStartSonicAnalysis (5 handlers)
[GIN-debug] GET    /rest/getSonicAnalysisStatus.view --> main.subsonicGetSonicAnalysisStatus (5 handlers)
[GIN-debug] GET    /rest/cancelSonicAnalysis.view --> main.subsonicCancelSonicAnalysis (5 handlers)
[GIN-debug] POST   /rest/cancelSonicAnalysis.view --> main.subsonicCancelSonicAnalysis (5 handlers)
[GIN-debug] PUT    /rest/cancelSonicAnalysis.view --> main.subsonicCancelSonicAnalysis (5 handlers)
[GIN-debug] PATCH  /rest/cancelSonicAnalysis.view --> main.subsonicCancelSonicAnalysis (5 handlers)
[GIN-debug] HEAD   /rest/cancelSonicAnalysis.view --> main.subsonicCancelSonicAnalysis (5 handlers)
[GIN-debug] OPTIONS /rest/cancelSonicAnalysis.view --> main.subsonicCancelSonicAnalysis (5 handlers)
[GIN-debug] DELETE /rest/cancelSonicAnalysis.view --> main.subsonicCancelSonicAnalysis (5 handlers)
[GIN-debug] CONNECT /rest/cancelSonicAnalysis.view --> main.subsonicCancelSonicAnalysis (5 handlers)
[GIN-debug] TRACE  /rest/cancelSonicAnalysis.view --> main.subsonicCancelSonicAnalysis (5 handlers)
[GIN-debug] GET    /rest/startSonicClustering.view --> main.subsonicStartClusteringAnalysis (5 handlers)
[GIN-debug] POST   /rest/startSonicClustering.view --> main.subsonicStartClusteringAnalysis (5 handlers)
[GIN-debug] PUT    /rest/startSonicClustering.view --> main.subsonicStartClusteringAnalysis (5 handlers)
[GIN-debug] PATCH  /rest/startSonicClustering.view --> main.subsonicStartClusteringAnalysis (5 handlers)
[GIN-debug] HEAD   /rest/startSonicClustering.view --> main.subsonicStartClusteringAnalysis (5 handlers)
[GIN-debug] OPTIONS /rest/startSonicClustering.view --> main.subsonicStartClusteringAnalysis (5 handlers)
[GIN-debug] DELETE /rest/startSonicClustering.view --> main.subsonicStartClusteringAnalysis (5 handlers)
[GIN-debug] CONNECT /rest/startSonicClustering.view --> main.subsonicStartClusteringAnalysis (5 handlers)
[GIN-debug] TRACE  /rest/startSonicClustering.view --> main.subsonicStartClusteringAnalysis (5 handlers)
[GIN-debug] POST   /api/v1/user/login        --> main.loginUser (4 handlers)
[GIN-debug] GET    /api/v1/user/me           --> main.userInfo (5 handlers)
[GIN-debug] GET    /api/v1/admin/browse      --> main.browseFiles (6 handlers)
[GIN-debug] POST   /api/v1/admin/scan/cancel --> main.cancelAdminScan (6 handlers)
[GIN-debug] POST   /api/v1/admin/scan/rescan --> main.rescanAllLibraries (6 handlers)
[GIN-debug] POST   /api/cleaning/start       --> main.CleaningStartHandler (6 handlers)
[GIN-debug] POST   /api/alchemy              --> main.AlchemyHandler (4 handlers)
[GIN-debug] GET    /api/map                  --> main.MapHandler (5 handlers)
[GIN-debug] GET    /api/voyager/search_tracks --> main.VoyagerSearchTracksHandler (5 handlers)
[GIN-debug] POST   /api/map/create_playlist  --> main.MapCreatePlaylistHandler (5 handlers)
[GIN-debug] GET    /static/*filepath         --> github.com/gin-gonic/gin.(*RouterGroup).createStaticHandler.func1 (4 handlers)
[GIN-debug] HEAD   /static/*filepath         --> github.com/gin-gonic/gin.(*RouterGroup).createStaticHandler.func1 (4 handlers)
[GIN-debug] GET    /favicon.ico              --> main.main.(*RouterGroup).StaticFile.func14 (4 handlers)
[GIN-debug] HEAD   /favicon.ico              --> main.main.(*RouterGroup).StaticFile.func14 (4 handlers)
[GIN-debug] GET    /manifest.json            --> main.main.(*RouterGroup).StaticFile.func15 (4 handlers)
[GIN-debug] HEAD   /manifest.json            --> main.main.(*RouterGroup).StaticFile.func15 (4 handlers)
```

## Compile and run frontend
Goining /music-server-frontend/

```
npm install
npm start
```

Frontend will be reacheable on http://localhost:3000/ you can do the first login with admin/admin

**IMPORTANT** as you can see, running the code OUT of the container, you had the front-end on the different port 3000




