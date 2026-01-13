import requests
import os
import logging

# Basic logging setup
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

class RiotAPI:
    def __init__(self, api_key):
        self.api_key = api_key
        self.base_url_account = "https://asia.api.riotgames.com" # For PUUID (VN is in Asia/SEA)
        self.base_url_match = "https://sea.api.riotgames.com"    # For Matches (VN/SEA servers)
        self.headers = {
            "X-Riot-Token": self.api_key
        }

    def get_puuid_by_riot_id(self, game_name, tag_line):
        """
        Get PUUID from Riot ID (Name#Tag)
        """
        url = f"{self.base_url_account}/riot/account/v1/accounts/by-riot-id/{game_name}/{tag_line}"
        try:
            response = requests.get(url, headers=self.headers)
            response.raise_for_status()
            return response.json().get("puuid")
        except requests.exceptions.RequestException as e:
            logger.error(f"Error fetching PUUID for {game_name}#{tag_line}: {e}")
            return None

    def get_match_ids_by_puuid(self, puuid, count=5):
        """
        Get list of recent match IDs
        """
        url = f"{self.base_url_match}/lol/match/v5/matches/by-puuid/{puuid}/ids"
        params = {"start": 0, "count": count}
        try:
            response = requests.get(url, headers=self.headers, params=params)
            response.raise_for_status()
            return response.json()
        except requests.exceptions.RequestException as e:
            logger.error(f"Error fetching match IDs for {puuid}: {e}")
            return []

    def get_match_details(self, match_id):
        """
        Get full details of a match
        """
        url = f"{self.base_url_match}/lol/match/v5/matches/{match_id}"
        try:
            response = requests.get(url, headers=self.headers)
            response.raise_for_status()
            return response.json()
        except requests.exceptions.RequestException as e:
            logger.error(f"Error fetching details for match {match_id}: {e}")
            return None

    def parse_match_data(self, full_data, target_puuid):
        """
        Filter and extract relevant data for AI analysis.
        Returns only the 5 teammates (including target) pre-filtered.
        """
        # Checks
        if not full_data or 'info' not in full_data:
            return None
        
        info = full_data['info']
        game_duration = info.get('gameDuration', 0)
        game_mode = info.get('gameMode', 'UNKNOWN')
        
        # Find the target player first to get their teamId
        target_player = None
        target_team_id = None
        participants_data = []
        
        participants = info.get('participants', [])
        
        # First pass: find target player and their team
        for p in participants:
            if p.get('puuid') == target_puuid:
                target_team_id = p.get('teamId')
                break
        
        if target_team_id is None:
            logger.warning("Target player not found in match participants")
            return None
        
        # Second pass: extract data for teammates only
        for p in participants:
            if p.get('teamId') != target_team_id:
                continue  # Skip enemies
            
            # Extract basic stats (only what AI needs)
            p_data = {
                'championName': p.get('championName'),
                'riotIdGameName': p.get('riotIdGameName'),
                'teamPosition': p.get('teamPosition'),
                'kills': p.get('kills'),
                'deaths': p.get('deaths'),
                'assists': p.get('assists'),
                'totalDamageDealtToChampions': p.get('totalDamageDealtToChampions'),
                'visionScore': p.get('visionScore'),
                'cs': p.get('totalMinionsKilled', 0) + p.get('neutralMinionsKilled', 0),
                'win': p.get('win'),
            }
            
            # Calculate KDA
            if p.get('deaths') == 0:
                p_data['kda'] = p.get('kills', 0) + p.get('assists', 0)
            else:
                p_data['kda'] = round((p.get('kills', 0) + p.get('assists', 0)) / p.get('deaths'), 2)

            participants_data.append(p_data)
            
            if p.get('puuid') == target_puuid:
                target_player = p_data

        return {
            'matchId': full_data.get('metadata', {}).get('matchId'),
            'gameDuration': game_duration,
            'gameMode': game_mode,
            'win': target_player.get('win') if target_player else None,
            'target_player': target_player,
            'teammates': participants_data  # Pre-filtered 5 teammates only
        }

