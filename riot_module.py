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
        Filter and extract relevant data for AI analysis
        """
        # Checks
        if not full_data or 'info' not in full_data:
            return None
        
        info = full_data['info']
        game_duration = info.get('gameDuration', 0)
        game_mode = info.get('gameMode', 'UNKNOWN')
        
        # Find the target player
        target_player = None
        participants_data = []
        
        participants = info.get('participants', [])
        for p in participants:
            # Extract basic stats
            p_data = {
                'championName': p.get('championName'),
                'teamPosition': p.get('teamPosition'),
                'kills': p.get('kills'),
                'deaths': p.get('deaths'),
                'assists': p.get('assists'),
                'kda': p.get('challenges', {}).get('kda', 0), # Some endpoints put kda in challenges
                'totalDamageDealtToChampions': p.get('totalDamageDealtToChampions'),
                'totalDamageTaken': p.get('totalDamageTaken'),
                'goldEarned': p.get('goldEarned'),
                'visionScore': p.get('visionScore'),
                'cs': p.get('totalMinionsKilled', 0) + p.get('neutralMinionsKilled', 0),
                'win': p.get('win'),
                'items': [p.get(f'item{i}') for i in range(7)],
                'puuid': p.get('puuid'),
                'riotIdGameName': p.get('riotIdGameName'),
                'riotIdTagline': p.get('riotIdTagline')
            }
            
            # Recalculate KDA if needed
            if p.get('deaths') == 0:
                p_data['kda_calculated'] = p.get('kills') + p.get('assists')
            else:
                p_data['kda_calculated'] = (p.get('kills') + p.get('assists')) / p.get('deaths')

            participants_data.append(p_data)
            
            if p.get('puuid') == target_puuid:
                target_player = p_data

        if not target_player:
            logger.warning("Target player not found in match participants")
            # Might return everything anyway, but highlighting target is useful
        
        return {
            'matchId': full_data.get('metadata', {}).get('matchId'),
            'gameDuration': game_duration,
            'gameMode': game_mode,
            'target_player': target_player,
            'all_players': participants_data # AI might need context of other players
        }
