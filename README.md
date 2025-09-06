# AudioMuse-AI-MusicServer
Prototype music server built on the Open Subsonic API, designed to showcase AudioMuse-AI’s advanced sonic analysis capabilities.

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

API will be reacheable on http://localhost:8080/api/v1/

## Compile and run frontend
Goin ing /music-server-frontend/

```
npm install
npm start
```

Frontend will be reacheable on http://localhost:3000/ you can do the first login with admin/admin
