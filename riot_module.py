import requests
import os
import json
import logging

# Basic logging setup
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

# Load champion data for role detection
def load_champion_data():
    """Load champion.json to get champion tags (Tank, Fighter, Mage, etc.)"""
    try:
        champion_file = os.path.join(os.path.dirname(__file__), 'champion.json')
        if os.path.exists(champion_file):
            with open(champion_file, 'r', encoding='utf-8') as f:
                data = json.load(f)
                return data.get('data', {})
    except Exception as e:
        logger.error(f"Error loading champion.json: {e}")
    return {}

CHAMPION_DATA = load_champion_data()

def get_champion_info(champion_name):
    """Get champion tags and stats from champion.json"""
    champ = CHAMPION_DATA.get(champion_name, {})
    return {
        'tags': champ.get('tags', []),  # e.g., ["Tank", "Fighter"]
        'defense': champ.get('info', {}).get('defense', 5),  # 1-10 scale
        'attack': champ.get('info', {}).get('attack', 5),
        'magic': champ.get('info', {}).get('magic', 5),
    }

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
        Filter and extract comprehensive data for AI analysis.
        Returns teammates and their lane opponents for comparison.
        """
        if not full_data or 'info' not in full_data:
            return None
        
        info = full_data['info']
        game_duration = info.get('gameDuration', 0)
        game_duration_minutes = game_duration / 60 if game_duration > 0 else 1
        game_mode = info.get('gameMode', 'UNKNOWN')
        
        # Find target player's team
        target_player = None
        target_team_id = None
        participants_data = []
        enemies_data = []
        
        participants = info.get('participants', [])
        
        for p in participants:
            if p.get('puuid') == target_puuid:
                target_team_id = p.get('teamId')
                break
        
        if target_team_id is None:
            logger.warning("Target player not found in match participants")
            return None
        
        def extract_player_data(p):
            """Helper function to extract player data"""
            challenges = p.get('challenges', {})
            champion_name = p.get('championName', '')
            champ_info = get_champion_info(champion_name)
            
            return {
                # Identity
                'championName': champion_name,
                'championTags': champ_info['tags'],  # ["Tank", "Fighter"] etc.
                'championDefense': champ_info['defense'],  # 1-10 scale
                'riotIdGameName': p.get('riotIdGameName'),
                'teamPosition': p.get('teamPosition'),
                'individualPosition': p.get('individualPosition'),
                'win': p.get('win'),
                
                # 1. Combat Performance
                'kills': p.get('kills'),
                'deaths': p.get('deaths'),
                'assists': p.get('assists'),
                'kda': round(challenges.get('kda', 0), 2),
                'killParticipation': round(challenges.get('killParticipation', 0) * 100, 1),
                'takedowns': challenges.get('takedowns', 0),
                'largestKillingSpree': p.get('largestKillingSpree', 0),
                'soloKills': challenges.get('soloKills', 0),
                'timeSpentDead': p.get('totalTimeSpentDead', 0),
                
                # 2. Damage Dealt
                'totalDamageDealtToChampions': p.get('totalDamageDealtToChampions', 0),
                'damagePerMinute': round(challenges.get('damagePerMinute', 0), 0),
                'teamDamagePercentage': round(challenges.get('teamDamagePercentage', 0) * 100, 1),
                'timeCCingOthers': p.get('timeCCingOthers', 0),
                
                # 3. Damage Taken (IMPORTANT for Tanks)
                'totalDamageTaken': p.get('totalDamageTaken', 0),
                'damageTakenOnTeamPercentage': round(challenges.get('damageTakenOnTeamPercentage', 0) * 100, 1),
                'damageSelfMitigated': p.get('damageSelfMitigated', 0),
                
                # 4. Laning & Economy
                'laneMinionsFirst10Minutes': challenges.get('laneMinionsFirst10Minutes', 0),
                'totalCS': p.get('totalMinionsKilled', 0) + p.get('neutralMinionsKilled', 0),
                'csPerMinute': round((p.get('totalMinionsKilled', 0) + p.get('neutralMinionsKilled', 0)) / game_duration_minutes, 1),
                'goldEarned': p.get('goldEarned', 0),
                'goldPerMinute': round(challenges.get('goldPerMinute', 0), 0),
                'champLevel': p.get('champLevel', 0),
                
                # 5. Macro & Objectives
                'dragonTakedowns': challenges.get('dragonTakedowns', 0),
                'baronTakedowns': challenges.get('baronTakedowns', 0),
                'damageDealtToObjectives': p.get('damageDealtToObjectives', 0),
                'turretTakedowns': challenges.get('turretTakedowns', 0),
                
                # 6. Vision Control
                'visionScore': p.get('visionScore', 0),
                'visionScorePerMinute': round(challenges.get('visionScorePerMinute', 0), 2),
                'wardsPlaced': p.get('wardsPlaced', 0),
                'controlWardsPlaced': challenges.get('controlWardsPlaced', 0),
                'wardsKilled': p.get('wardsKilled', 0),
            }
        
        # Extract data for both teams
        for p in participants:
            p_data = extract_player_data(p)
            
            if p.get('teamId') == target_team_id:
                participants_data.append(p_data)
                if p.get('puuid') == target_puuid:
                    target_player = p_data
            else:
                enemies_data.append(p_data)
        
        # Match lane opponents
        position_map = {'TOP': 'TOP', 'JUNGLE': 'JUNGLE', 'MIDDLE': 'MIDDLE', 'BOTTOM': 'BOTTOM', 'UTILITY': 'UTILITY'}
        lane_matchups = []
        
        for teammate in participants_data:
            pos = teammate.get('teamPosition', '')
            opponent = None
            for enemy in enemies_data:
                if enemy.get('teamPosition', '') == pos:
                    opponent = enemy
                    break
            
            lane_matchups.append({
                'player': teammate,
                'opponent': opponent
            })

        return {
            'matchId': full_data.get('metadata', {}).get('matchId'),
            'gameDuration': game_duration,
            'gameDurationMinutes': round(game_duration_minutes, 1),
            'gameMode': game_mode,
            'win': target_player.get('win') if target_player else None,
            'target_player_name': target_player.get('riotIdGameName') if target_player else None,
            'teammates': participants_data,
            'lane_matchups': lane_matchups
        }
