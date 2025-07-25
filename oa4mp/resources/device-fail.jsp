<%@ page contentType="text/html;charset=UTF-8" language="java" %>
<%@ page session="false" %>
<html>
<head>
  <title>Pelican Consent Page</title>
  <link rel="preconnect" href="https://fonts.googleapis.com">
  <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
  <link href="https://fonts.googleapis.com/css2?family=Poppins:ital@0;1&display=swap" rel="stylesheet">
  <style>
    body {
      font-family: 'Poppins', 'Helvetica Neue', Arial, sans-serif;
      background-color: #ffffff;
      margin: 0;
      padding: 0;
      position: relative;
      overflow: hidden;
      display: flex;
      flex-direction: column;
      align-items: center;
      justify-content: center;
      height: 100vh;
    }
    .background-effect {
      position: absolute;
      top: 0;
      left: 0;
      width: 100%;
      height: 100%;
      background: conic-gradient(from 180deg at 80% 80%, #16abff 0deg, #0885ff 55deg, #54d6ff 120deg, #0071ff 160deg, #0071ff 1turn);
      opacity: 20%;
      z-index: -1;
      filter: blur(20px);
    }
    .pelican-header {
      background-color: #0885ff;
      color: white;
      padding: 20px;
      text-align: center;
    }
    .pelican-logo {
      vertical-align: middle;
    }
    h1 {
      margin: 0;
      font-size: 24px;
    }
    h2 {
      color: #0885ff;
      text-align: center;
    }
    p {
      text-align: center;
      font-size: 16px;
    }
    .pelican-button {
      background-color: #0885ff;
      color: white;
      border: none;
      padding: 10px 20px;
      cursor: pointer;
      font-size: 14px;
      border-radius: 5px;
    }
    .pelican-button:hover {
      background-color: #005bb5;
    }
    a {
      text-decoration: none;
    }
    .content-container {
      display: flex;
      flex-direction: column;
      align-items: center;
      margin: 20px;
    }
    .client-info {
      background-color: white;
      padding: 20px;
      border-radius: 5px;
      box-shadow: 0 2px 10px rgba(0, 0, 0, 0.1);
      margin-bottom: 20px;
    }
    .button-container {
      display: flex;
      justify-content: center;
      gap: 10px;
      margin-bottom: 20px;
    }
    .footer-logo {
      text-align: center;
      margin-top: 40px;
    }
  </style>
</head>
<body>
<div class="background-effect"></div>
<main>
  <div class="content-container">
    <h2>User Code Denied!</h2>
    <p>You have exceeded the number of retry attempts for entering a code.</p>
  </div>
</main>
<div class="footer-logo">
  <img height="80" src="https://pelicanplatform.org/static/images/PelicanPlatformLogo_Icon.png" alt="Pelican Logo">
</div>
</body>
</html>
